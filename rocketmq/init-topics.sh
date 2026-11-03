#!/usr/bin/env bash
set -euo pipefail

# 在 broker 容器内执行，把业务 topic 与消费组补齐。
# 用法：docker exec feed-broker sh /home/rocketmq/init-topics.sh

MQADMIN="/home/rocketmq/rocketmq-5.3.2/bin/mqadmin"
NAMESRV_ADDR="${NAMESRV_ADDR:-namesrv:9876}"
CLUSTER="${CLUSTER_NAME:-DefaultCluster}"

TOPICS=(
  video_created
  video_deleted
  video_private_switched
  comment_created
  comment_deleted
  like_switched
  favorite_switched
  follow_switched
  user_updated
)

GROUPS=(
  feed_video_user
  feed_video_follow
  feed_video_video
  feed_video_comment
  feed_video_feed
  feed_video_like
  feed_video_favorite
  feed_video_message
)

for topic in "${TOPICS[@]}"; do
  for name in "$topic" "${topic}_failed"; do
    sh "$MQADMIN" updateTopic -n "$NAMESRV_ADDR" -c "$CLUSTER" -t "$name" -r 4 -w 4
  done
done

for group in "${GROUPS[@]}"; do
  sh "$MQADMIN" updateSubGroup -n "$NAMESRV_ADDR" -c "$CLUSTER" -g "$group"
done

sh "$MQADMIN" topicList -n "$NAMESRV_ADDR"
