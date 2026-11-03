<template>
  <li class="message-item" :class="{ unread: !message.isRead }" @click="$emit('open', message)">
    <span class="unread-dot" aria-hidden="true"></span>

    <img
      class="actor-avatar"
      :src="primaryActor.avatar || fallbackAvatar"
      :alt="primaryActor.nickname"
      title="进入主页"
      @click.stop="$emit('open-actor', primaryActor.id)"
    />

    <div class="message-body">
      <p class="message-title">{{ message.title }}</p>
      <p class="message-text">{{ summary }}</p>
      <p class="message-time">{{ timeText }}</p>
    </div>

    <img v-if="message.coverUrl" class="message-cover" :src="message.coverUrl" alt="目标封面" />
  </li>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { MessageActor, MessageItem } from "@/types/domain";

const props = defineProps<{ message: MessageItem }>();
defineEmits<{
  (e: "open", message: MessageItem): void;
  // open-actor 点头像进触发者的主页，与整行点击的跳转分开
  (e: "open-actor", actorId: number): void;
}>();

const fallbackAvatar = "https://feed-cp.oss-cn-beijing.aliyuncs.com/avatar/default.svg";

const primaryActor = computed<MessageActor>(() => {
  const first = props.message.actors[0];
  return first ?? { id: 0, nickname: "有人", avatar: "" };
});

// 文案：一个人和多个人的说法不同，多人时用"等 N 人"
const summary = computed(() => {
  const actorName = primaryActor.value.nickname;
  const others = props.message.actorCount - 1;
  const who = others > 0 ? `${actorName} 等 ${props.message.actorCount} 人` : actorName;
  const action = actionText(props.message.type);
  const content = props.message.content ? `：${props.message.content}` : "";
  return `${who} ${action}${content}`;
});

const timeText = computed(() => relativeTime(props.message.createdAt));

function actionText(type: string) {
  if (type === "like_video") return "赞了你的视频";
  if (type === "like_comment") return "赞了你的评论";
  if (type === "comment") return "评论了你的视频";
  if (type === "reply") return "回复了你的评论";
  if (type === "follow") return "关注了你";
  return "给你发来一条消息";
}

function relativeTime(value: string) {
  const at = new Date(value).getTime();
  if (!Number.isFinite(at)) return "";
  const diff = Date.now() - at;
  if (diff < 60_000) return "刚刚";
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`;
  if (diff < 7 * 86_400_000) return `${Math.floor(diff / 86_400_000)} 天前`;
  const date = new Date(at);
  return `${date.getMonth() + 1} 月 ${date.getDate()} 日`;
}
</script>

<style scoped>
.message-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.message-item.unread {
  background-color: rgba(255, 255, 255, 0.04);
}

.unread-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: transparent;
  flex-shrink: 0;
}

.message-item.unread .unread-dot {
  background-color: var(--tiktok-red);
}

.actor-avatar {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  object-fit: cover;
  flex-shrink: 0;
  background-color: #1f1f1f;
}

.message-body {
  flex: 1;
  min-width: 0;
}

.message-title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.72);
}

.message-text {
  margin: 3px 0 0;
  font-size: 14px;
  color: #ffffff;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-time {
  margin: 4px 0 0;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.45);
}

.message-cover {
  width: 46px;
  height: 60px;
  border-radius: 6px;
  object-fit: cover;
  flex-shrink: 0;
  background-color: #1f1f1f;
}
</style>
