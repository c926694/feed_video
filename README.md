# 简易版短视频平台部署说明

## 服务说明

当前 `docker-compose.yml` 会启动这些服务：

- `web`：前端 Nginx，宿主机端口 `81`，容器端口 `81`
- `backend`：Go HTTP 服务，宿主机端口 `8081`，容器端口 `8081`
- `mysql`：宿主机端口 `3307`，容器端口 `3307`
- `redis`：宿主机端口 `6380`，容器端口 `6380`
- `namesrv`：RocketMQ NameServer，宿主机端口 `9876`，容器端口 `9876`
- `broker`：RocketMQ Broker 内嵌 Proxy，宿主机端口 `10911`（remoting）与 `18080`（gRPC），
  容器端口同为 `10911` 与 `18080`

容器内部服务名和 `back/config/config.compose.example.yaml` 保持一致：

- MySQL：`mysql:3307`
- Redis：`redis:6380`
- RocketMQ：`broker:18080`（Proxy 的 gRPC 地址，也就是 `rocketmq.endpoint`）
- Backend：`backend:8081`

RocketMQ 的 topic 与消费组需要在部署后执行一次初始化。镜像里的 `/bin/sh` 是 dash，
而脚本用了 `set -o pipefail`，必须用 bash 执行：

```bash
docker exec feed-broker bash /home/rocketmq/init-topics.sh
```

初始化脚本会创建 7 个业务 topic、7 个 `<topic>_failed` 留档 topic 与 5 个消费组。
RocketMQ 的 topic 名只允许字母数字与下划线，所以失败留档 topic 用下划线而非点号。

## 自动部署

可以直接使用 `deploy.sh`：

```bash
chmod +x deploy.sh
./deploy.sh
```

脚本会依次执行：

1. 校验 `docker compose` 配置
2. 拉取基础镜像
3. 构建项目镜像
4. 启动所有服务
5. 输出容器状态

如果有容器启动失败，脚本会自动输出最近的 compose 日志并返回非零退出码。

## 访问方式

- 前端首页：`http://localhost:81`
- 后端接口入口：`http://localhost:8081`
- 静态资源通过前端 Nginx 转发：`http://localhost:81/static/...`