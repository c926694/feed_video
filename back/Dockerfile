# syntax=docker.m.daocloud.io/docker/dockerfile:1.7
FROM docker.m.daocloud.io/library/golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
# 国内网络下通过 goproxy.cn 拉取 Go 依赖
ENV GOPROXY=https://goproxy.cn,direct
ARG TARGETOS=linux
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/http ./cmd/http
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/listener ./cmd/listener

FROM docker.m.daocloud.io/library/alpine:3.20

WORKDIR /app

# 改用阿里云镜像站安装运行时依赖
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/http /app/http
COPY --from=builder /out/listener /app/listener
COPY config/config.compose.example.yaml /app/config/config.yaml
COPY assets/default-avatar.svg /app/defaults/avatar/default.svg
COPY scripts/start-http.sh /app/start-http.sh
COPY scripts/start-listener.sh /app/start-listener.sh

RUN chmod +x /app/start-http.sh /app/start-listener.sh \
    && sed -i 's/\r$//' /app/start-http.sh /app/start-listener.sh

EXPOSE 8081

CMD ["/app/start-http.sh"]
