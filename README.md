# 简易版短视频平台部署说明

## 服务说明

当前 `docker-compose.yml` 会启动这些服务：

- `web`：前端 Nginx，宿主机端口 `81`，容器端口 `81`
- `backend`：Go HTTP 服务，宿主机端口 `8081`，容器端口 `8081`
- `listener`：Kafka 消费者，无需对外暴露端口
- `mysql`：宿主机端口 `3307`，容器端口 `3307`
- `redis`：宿主机端口 `6380`，容器端口 `6380`
- `kafka`：宿主机端口 `9092`，容器端口 `9092`

容器内部服务名和 `back/config/config.compose.example.yaml` 保持一致：

- MySQL：`mysql:3307`
- Redis：`redis:6380`
- Kafka：`kafka:9092`
- Backend：`backend:8081`

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