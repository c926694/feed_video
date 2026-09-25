# syntax=docker.m.daocloud.io/docker/dockerfile:1.7
FROM docker.m.daocloud.io/library/node:20-alpine AS builder

WORKDIR /app

COPY package*.json ./
# 国内网络下通过 npmmirror 拉取依赖，并复用 npm 缓存
RUN --mount=type=cache,target=/root/.npm \
    npm ci --registry=https://registry.npmmirror.com --no-audit --no-fund

COPY . .
RUN npm run build

FROM docker.m.daocloud.io/library/nginx:1.27-alpine

COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 81
