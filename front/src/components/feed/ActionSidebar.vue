<template>
  <aside class="sidebar">
    <button v-if="showFollow" class="avatar-btn" @click="$emit('toggle-follow')">
      <img v-if="video.author.avatar" :src="video.author.avatar" alt="avatar" />
      <span v-else>{{ video.author.nickname.slice(0, 1) }}</span>
      <b>{{ video.followed ? "已关注" : "+关注" }}</b>
    </button>

    <button class="action-btn like" :class="{ active: video.liked }" @click="$emit('toggle-like')">
      <span class="icon-wrap" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="icon-svg">
          <path
            d="M12 20.2 4.9 13.7a4.9 4.9 0 0 1 6.9-7L12 7.9l.2-.2a4.9 4.9 0 0 1 6.9 7L12 20.2Z"
            class="heart-fill"
          />
          <path
            d="M12 20.2 4.9 13.7a4.9 4.9 0 0 1 6.9-7L12 7.9l.2-.2a4.9 4.9 0 0 1 6.9 7L12 20.2Z"
            class="heart-stroke"
          />
        </svg>
      </span>
      <small>{{ video.likeCount }}</small>
    </button>

    <button class="action-btn fav" :class="{ active: video.favorited }" @click="$emit('toggle-favorite')">
      <span class="icon-wrap" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="icon-svg">
          <path
            d="m12 3 2.7 5.7 6.3.8-4.6 4.3 1.2 6.2L12 17l-5.6 3 1.2-6.2L3 9.5l6.3-.8Z"
            class="star-fill"
          />
          <path
            d="m12 3 2.7 5.7 6.3.8-4.6 4.3 1.2 6.2L12 17l-5.6 3 1.2-6.2L3 9.5l6.3-.8Z"
            class="star-stroke"
          />
        </svg>
      </span>
      <small>{{ video.favoriteCount }}</small>
    </button>

    <button class="action-btn" @click="$emit('comment')">
      <span class="icon-wrap" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="icon-svg">
          <path d="M6 7h12a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2H11l-4.5 3V18H6a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2Z" />
        </svg>
      </span>
      <small>{{ video.commentCount }}</small>
    </button>

    <button class="action-btn" @click="$emit('share')">
      <span class="icon-wrap" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="icon-svg">
          <path d="M14 5 19 5 19 10" />
          <path d="M19 5 11 13" />
          <path d="M19 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h4" />
        </svg>
      </span>
      <small>分享</small>
    </button>

    <button v-if="showDelete" class="action-btn danger" @click="$emit('delete-video')">
      <span class="icon-wrap" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="icon-svg">
          <path d="M4 7h16" />
          <path d="M10 11v6" />
          <path d="M14 11v6" />
          <path d="M8 7l1-2h6l1 2" />
          <path d="M7 7v11a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2V7" />
        </svg>
      </span>
      <small>删除</small>
    </button>
  </aside>
</template>

<script setup lang="ts">
import type { Video } from "@/types/domain";

withDefaults(
  defineProps<{
    video: Video;
    showFollow?: boolean;
    showDelete?: boolean;
  }>(),
  {
    showFollow: true,
    showDelete: false
  }
);

defineEmits<{
  (e: "toggle-like"): void;
  (e: "toggle-favorite"): void;
  (e: "comment"): void;
  (e: "toggle-follow"): void;
  (e: "share"): void;
  (e: "delete-video"): void;
}>();
</script>

<style scoped>
.sidebar {
  position: absolute;
  right: 10px;
  bottom: 136px;
  z-index: 8;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 8px 6px;
  border-radius: 20px;
  background: rgba(0, 0, 0, 0.26);
  backdrop-filter: blur(6px);
}

button {
  border: none;
  background: transparent;
  color: #fff;
}

.avatar-btn {
  position: relative;
  width: 50px;
  height: 50px;
  border-radius: 50%;
  overflow: hidden;
  border: 2px solid #fff;
  padding: 0;
  display: grid;
  place-items: center;
  background: #2a3043;
}

.avatar-btn img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-btn span {
  font-size: 20px;
}

.avatar-btn b {
  position: absolute;
  bottom: -8px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 12px;
  line-height: 1;
  background: var(--accent);
  border-radius: 10px;
  padding: 2px 6px;
  white-space: nowrap;
}

.action-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  cursor: pointer;
  padding: 3px 2px;
  border-radius: 14px;
  background: rgba(0, 0, 0, 0.26);
}

.icon-wrap {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
}

.icon-svg {
  width: 24px;
  height: 24px;
  stroke: currentColor;
  stroke-width: 1.8;
  fill: none;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.heart-fill {
  fill: currentColor;
  opacity: 0;
  stroke: none;
}

.heart-stroke {
  fill: none;
}

.action-btn.like.active {
  color: #ff5d7a;
}

.action-btn.like.active .heart-fill {
  opacity: 1;
}

.star-fill {
  fill: currentColor;
  opacity: 0;
  stroke: none;
}

.star-stroke {
  fill: none;
}

.action-btn.fav.active {
  color: #ffc23d;
}

.action-btn.fav.active .star-fill {
  opacity: 1;
}

.action-btn.danger {
  color: #ff6b6b;
}

small {
  font-size: 12px;
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.6);
}
</style>
