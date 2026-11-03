# Kafka 换成 RocketMQ（gRPC 客户端）

- 时间：2026-11-03
- 范围：`feed_video` 仓库的消息层、配置、部署文件
- 目标环境：`192.168.145.101`

## 问题与根因

原来 5 个模块、7 个 topic、16 个订阅全部建立在 Kafka 上，客户端是 `github.com/segmentio/kafka-go v0.4.47`：

- 生产者按 topic 复用 `kafka.Writer`，消费端用 `kafka.NewReader` 手动 `CommitMessages`；
- 消费组名是 `feed_video.<模块名>.<topic名>`；
- 处理失败本地重试 3 次，仍失败转发到 `<topic>.failed`。

要求换成 RocketMQ 并指定使用 gRPC 客户端（`rocketmq-clients/golang/v5`），因此整条链路按 5.x 的 Proxy 模型重建。

## 改了什么

### 代码

- 新增 `back/internal/platform/mq/`：`producer`、`consumer`、`topic` 三个包，删除 `back/internal/platform/kafka/`；
- 生产者：`rmq.NewProducer` + `rmq.WithTopics(...)`，`SendSync` 发送，`GracefulStop` 关闭；
- 消费者：一个模块一个 `PushConsumer`，组名 `feed_video_<模块名>`，`WithPushSubscriptionExpressions` 一次登记该模块的全部 topic，回调按 `MessageView.GetTopic()` 分发；
- 失败语义：`consumer.Permanent` 的错误转发到 `<topic>_failed` 后返回 `SUCCESS`；其余错误返回 `FAILURE` 由服务端重投，重投耗尽进入 `%DLQ%<分组>`；
- 13 个模块文件的 import 从 `platform/kafka` 改为 `platform/mq`；
- `back/internal/platform/config/config.go`：`KafkaConfig` 换成 `RocketMQConfig`（`endpoint` / `access_key` / `access_secret` / `group_id`）；
- `back/cmd/server/main.go`：生产者与消费者按新签名装配，生产者传入全部业务 topic。

### 配置

`back/config/config.compose.example.yaml`：

```yaml
rocketmq:
  endpoint: broker:18080
  access_key: ""
  access_secret: ""
  group_id: feed_video
```

### 部署

`docker-compose.yml`：kafka 服务换成 `namesrv` + `broker`（`mqbroker --enable-proxy` 的 ProxyStartup local 模式，同一进程同时提供 10911 remoting 与 18080 gRPC）。

新增 `rocketmq/` 下三个文件：

- `broker.conf`：`brokerIP1=192.168.145.101`、`autoCreateTopicEnable=true`、`autoCreateSubscriptionGroup=true`；
- `proxy.conf`：JSON 格式，`{"grpcServerPort": 18080}`；
- `init-topics.sh`：创建 7 个业务 topic、7 个 `<topic>_failed`、5 个消费组。

### 依赖

`back/go.mod` 去掉 `github.com/segmentio/kafka-go v0.4.47`，加入 `github.com/apache/rocketmq-clients/golang/v5 v5.1.4`，连带引入 `google.golang.org/grpc v1.72.0`、`google.golang.org/protobuf v1.36.6`、`go.uber.org/zap v1.27.0`、`go.opencensus.io v0.24.0` 等。

## 实测验证数据

在 `192.168.145.101` 上完整跑通：

| 项目 | 结果 |
|---|---|
| 镜像 | `apache/rocketmq:5.3.2`，592MB |
| `feed-namesrv` | Up（`HEAP_OPTS=-Xms128m -Xmx128m -Xmn64m`） |
| `feed-broker` | Up healthy（`HEAP_OPTS=-Xms512m -Xmx512m -Xmn256m -XX:MaxDirectMemorySize=256m`） |
| proxy gRPC | `grpc server has built. port: 18080`、`rocketmq-proxy startup successfully` |
| 裸 gRPC 探测 | `QueryRoute` 返回 `CLIENT_ID_REQUIRED`，说明 HTTP/2 通路正常 |
| topic 初始化 | 7 个业务 topic + 7 个 `*_failed` + 5 个消费组全部创建成功 |
| 消费组启动 | `feed_video_user topics=3`、`feed_video_comment topics=3`、`feed_video_feed topics=5`、`feed_video_video topics=5`、`feed_video_follow topics=1` |
| 端到端 | `mqadmin sendMessage -t video_created` 返回 `SEND_OK`，`feed` 与 `user` 两个模块各自打印 `消息处理完成 topic=video_created` |

### 迁移中踩到并已修掉的坑

1. topic 名与消费组名只允许 `^[%|a-zA-Z0-9_-]+$`，原来的 `<topic>.failed` 与 `feed_video.user.video_created` 都非法，失败 topic 改成 `<topic>_failed`，组名改成 `feed_video_<模块名>`。
2. gRPC 客户端 `Start()` 会按 `WithTopics` 的列表拉取路由并同步 settings；列表为空时 `syncSettings` 永远等不到结果，进程卡在「wait for sync settings finish」。生产者必须传入全部 topic。
3. Proxy 的 gRPC 默认端口是 `8081`，宿主机 `8081` 已被 backend 占用；同时 Proxy 通告给客户端的地址必须与客户端实际连接的地址一致，所以把 `grpcServerPort` 设成 `18080`，宿主机按同名端口映射 `18080:18080`。
4. `-pc` 指定的 proxy 配置必须是 JSON，写成 properties 会触发 `fastjson.JSONException: syntax error`。
5. 把命名卷挂到 `/home/rocketmq/logs` 会覆盖镜像内的目录，logback 无法创建 `rocketmqlogs` 子目录，broker 反复重启（`Failed to create parent directories`）。
6. 镜像 `/bin/sh` 是 dash，`init-topics.sh` 用了 `set -o pipefail`，必须用 bash 执行。
7. 镜像默认堆是 broker 2g / namesrv 1g，2.4G 内存的机器必须用 `HEAP_OPTS` 压到 512m / 128m。

## 影响面

- 代码：消息层 3 个包重写，13 个模块文件只改 import；业务 Handler 签名（`func(ctx, []byte) error`）不变；
- 配置：`kafka` 段整体替换为 `rocketmq` 段，旧配置不再被读取；
- 部署：`kafka` 服务移除，新增 `namesrv`、`broker`；`backend` 的 `depends_on` 从 kafka 改为 broker；
- 依赖：`kafka-go` 移除，新增 grpc/protobuf/zap/opencensus 等间接依赖，`go.sum` 体积增大；
- 数据：Kafka 中的历史消息不迁移，RocketMQ 的 topic 需要执行一次 `init-topics.sh`；
- 语义：可重试错误由服务端重投（默认 16 次）而不是本地重试 3 次；热度用 `ZINCRBY` 累加，重投会重复计分，与原先「处理成功后提交位点失败」的重放风险同类。

## 回滚方式

1. 代码：手动把 `back/internal/platform/kafka/` 三个包恢复，13 个模块文件与 `main.go`、`svc/context.go`、`config/config.go` 的 import 与字段改回，`go.mod` 恢复 `kafka-go`；
2. 配置：两份 yaml 的 `rocketmq` 段改回 `kafka` 段；
3. 部署：`docker-compose.yml` 的 `namesrv`/`broker` 换回 `kafka` 服务，删除 `rocketmq/` 目录；
4. 101 上的备份：`/opt/feed_video/docker-compose.yml.bak-kafka-20261103`、`/opt/feed_video/back/config/config.yaml.bak-kafka-20261103`。

## 遗留项

1. `brokerIP1=192.168.145.101` 是写死的，换机器部署要同步改 `rocketmq/broker.conf`。
2. 生产建议用 `mqadmin updateTopic` 显式建 topic 并指定队列数，不依赖 `autoCreateTopicEnable`。

## 合并对方分支（1b00f9d「部署」）的处理

`d9c0537` 之后执行了 `git merge 1b00f9d`，产生 12 个文件冲突与 3 个 `DU` 的文件，处理方式如下：

- 消息层：删除 `back/internal/platform/kafka/{consumer,producer,topic}`，全部改用 `back/internal/platform/mq`；
- 12 个冲突文件：`config.go` 保留 `RocketMQConfig` 并合入对方的 `RAMConfig`；`svc/context.go` 用 `mq/producer` 并保留对方的 `sts`；`comment/router.go`、`user/router.go` 保留对方的 `videorepo` 并用 `mq`；`follow/logic.go`、`like/logic.go`、`video/logic.go` 用 `mq` 并保留对方新增的 `upload`、`sts`；`go.mod` 同时保留 `rocketmq-clients`、`alibaba-cloud-sdk-go`、`golang-migrate` 与对方的 `opentracing`；`go.sum` 由 `go mod tidy` 重新生成；`README.md` 按对方的五节结构重写并替换为 RocketMQ 内容；`docker-compose.yml` 保留对方的 `container_name`、backend 健康检查与 RAM 环境变量，服务段换成 namesrv + broker；
- 对方新增的 `favorite`、`message` 模块以及 `like/router.go` 的 import 一并改成 `platform/mq`；
- topic 扩到 9 个（新增 `video_private_switched`、`favorite_switched`），消费组扩到 8 个（新增 `feed_video_like`、`feed_video_favorite`、`feed_video_message`），`rocketmq/init-topics.sh` 同步更新；
- 验证：`go build ./...` 与 `go vet ./...` 均通过，`git status` 没有未解决冲突，合并尚未提交。
