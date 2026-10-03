# 短视频平台部署说明

整套服务用 Docker Compose 启动：MySQL、Redis、Kafka、后端（HTTP 与 Kafka 消费者在同一个进程里）、前端 nginx。容器之间一律用服务名互访，不写 IP。

## 一、前置要求

- 装好 Docker 与 Docker Compose（`docker --version`、`docker compose version` 都能输出即可）
- 一对阿里云 OSS 凭据（AccessKey ID 与 Secret）与要用的 bucket 名
- 一对 RAM 子账号凭据与一个角色：子账号只授 `sts:AssumeRole` 权限，角色上挂允许 `oss:PutObject` 与分片相关动作的策略，并把子账号加进角色的信任策略。这一对凭据用来给前端签发直传 OSS 的临时凭证

第二项缺了服务也能起来，只是发布视频走不通：上传凭证接口会返回「直传服务未配置」。

## 二、三步启动

```bash
# 1. 复制配置
cp back/config/config.compose.example.yaml back/config/config.yaml

# 2. 编辑 back/config/config.yaml，填这些必填项：
#    mysql.password                                 与 docker-compose.yml 里 mysql 的 MYSQL_ROOT_PASSWORD 保持一致
#    jwt.secret                                     自己的随机串
#    upload.oss.bucket                              你的 bucket 名
#    upload.oss.access_key_id / access_key_secret   OSS 凭据
#    ram.region_id                                  例如 cn-beijing
#    ram.role_arn                                   被扮演的角色 ARN
#    ram.access_key_id / access_key_secret          RAM 子账号凭据

# 3. 起服务（首次会构建镜像，需要几分钟）
docker compose up -d
```

四处凭据也可以留空，改用环境变量注入，compose 会把它们传给后端：`OSS_ACCESS_KEY_ID`、`OSS_ACCESS_KEY_SECRET`、`RAM_ACCESS_KEY_ID`、`RAM_ACCESS_KEY_SECRET`。留空时以配置文件里的值为准。

代码改动之后要重新构建镜像：`docker compose up -d --build`。

## 三、服务与端口

- `web`：前端 nginx，宿主机 `81`
- `backend`：Go 服务，HTTP 与 Kafka 消费者在同一个进程，宿主机 `8081`
- `mysql`：宿主机 `3307`
- `redis`：宿主机 `6380`
- `kafka`：容器之间用 `9092`；宿主机上的命令行工具用 `19092`（对外监听器）

## 四、访问与排查

- 前端页面：`http://localhost:81`
- 后端接口：`http://localhost:8081`，页面通过 nginx 的 `/api/` 反代访问，同源，不涉及跨域
- 存活探测：`http://localhost:8081/healthz`

仓库里没有种子数据，第一次跑起来需要在前端注册一个账号。数据库表由后端启动时自动迁移，不需要单独执行迁移命令。

```bash
docker compose ps                    # 容器与健康状态
docker compose logs -f backend       # 后端日志，含 Kafka 消费日志
docker compose logs --tail=100 web   # nginx 日志
docker compose down                  # 停止并删除容器，数据卷保留
docker compose down -v               # 连数据卷一起删，下次启动是空库
```

几种常见情况：

- 后端起不来并打印「加载配置失败」：`back/config/config.yaml` 不存在或者格式不对，先确认第二步做过
- 上传凭证接口返回「直传服务未配置」：`ram` 段没填全，或者角色与子账号的信任关系没配好
- 页面能打开但接口全部 401：`jwt.secret` 与 `access_token_minutes`、`refresh_token_hours` 没填好，令牌有效期为 0
- 从别的机器访问页面：把 `KAFKA_EXTERNAL_HOST` 设成那台机器能解析的地址再起服务
- 新加一个 topic 的订阅之后消费不动：消费方先启动而 topic 还不存在时，消费组拿不到分区，重启一次 backend 即可

## 五、可选：一键脚本

`deploy.sh` 把上面的流程串了一遍：校验 compose 配置、拉基础镜像、构建、启动、打印状态，任一容器没起来就输出最近的日志并以非零码退出。

```bash
chmod +x deploy.sh
./deploy.sh
```
