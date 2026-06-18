# 简易版短视频平台部署说明

## 注意事项

1. **端口映射**  
   - web: `81:80`  
   - backend: `8081:8080`  
   - MySQL: `3307:3306`  
   - Redis: `6380:6379`  
   - Kafka: `9093:9092`  
   - Zookeeper: `2181:2181`

2. **自动部署脚本**  
   - 使用 `deploy.sh` 可以一键构建并启动所有服务  
   - 确保脚本有执行权限：

     ```bash
     chmod +x deploy.sh
     ./deploy.sh
     ```
