#!/usr/bin/env bash
set -euo pipefail

# Broker 与 Proxy 在同一个进程里启动。主进程交给 exec，停止时信号能正常送达；
# 初始化放到后台子进程里，等 Broker 在 NameServer 注册完成后再建 topic 与消费组。
MQADMIN="/home/rocketmq/rocketmq-5.3.2/bin/mqadmin"
NAMESRV_ADDR="${NAMESRV_ADDR:-namesrv:9876}"

(
  for _ in $(seq 1 60); do
    if sh "$MQADMIN" clusterList -n "$NAMESRV_ADDR" 2>/dev/null | grep -q broker-a; then
      break
    fi
    sleep 2
  done
  bash /home/rocketmq/init-topics.sh > /home/rocketmq/init-topics.log 2>&1
) &

exec sh mqbroker --enable-proxy -c /home/rocketmq/broker.conf -pc /home/rocketmq/proxy.conf
