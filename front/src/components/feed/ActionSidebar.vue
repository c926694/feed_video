<template>
  <aside class="sidebar">
    <div v-if="showFollow" class="avatar-wrap">
      <button class="avatar-btn" title="进入主页" @click="$emit('open-profile')">
        <img v-if="video.author.avatar" :src="video.author.avatar" alt="avatar" />
        <span v-else class="avatar-initial">{{ (video.author.nickname || video.author.username || "匿").slice(0, 1) }}</span>
      </button>
      <button
        class="follow-badge"
        :class="{ followed: video.followed }"
        :title="video.followed ? '已关注' : '关注作者'"
        @click.stop="$emit('toggle-follow')"
      >
        <span v-if="video.followed" class="check-icon">✓</span>
        <span v-else class="plus-icon">+</span>
      </button>
    </div>

    <button class="action-btn like-btn" :class="{ active: video.liked }" @click="$emit('toggle-like')">
      <span class="icon-bubble" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="action-svg">
          <path
            d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"
          />
        </svg>
      </span>
      <small class="count-label">{{ video.likeCount }}</small>
    </button>

    <button class="action-btn fav-btn" :class="{ active: video.favorited }" @click="$emit('toggle-favorite')">
      <span class="icon-bubble" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="action-svg">
          <path
            d="m12 2.5 3.09 6.26 6.91 1-5 4.87 1.18 6.88L12 18.25l-6.18 3.26L7 14.63l-5-4.87 6.91-1L12 2.5z"
          />
        </svg>
      </span>
      <small class="count-label">{{ video.favoriteCount }}</small>
    </button>

    <button class="action-btn comment-btn" @click="$emit('comment')">
      <span class="icon-bubble" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="action-svg">
          <path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2z" />
        </svg>
      </span>
      <small class="count-label">{{ video.commentCount }}</small>
    </button>

    <button class="action-btn share-btn" @click="$emit('share')">
      <span class="icon-bubble" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="action-svg">
          <path d="m14 9-1.41 1.41L15.17 13H4v2h11.17l-2.58 2.59L14 19l6-6-6-6z" />
        </svg>
      </span>
      <small class="count-label">分享</small>
    </button>

    <button
      v-if="showPrivate"
      class="action-btn private-btn"
      :class="{ active: video.status === 'private' }"
      @click="$emit('toggle-private')"
    >
      <span class="icon-bubble" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="action-svg">
          <path
            v-if="video.status === 'private'"
            d="M12 2a5 5 0 0 0-5 5v3H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-1V7a5 5 0 0 0-5-5zm-3 8V7a3 3 0 0 1 6 0v3H9z"
          />
          <path
            v-else
            d="M12 2a5 5 0 0 0-5 5v3H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-1V7a5 5 0 0 0-5-5zm0 2a3 3 0 0 1 3 3v3H9V7a3 3 0 0 1 3-3z"
          />
        </svg>
      </span>
      <small class="count-label">{{ video.status === "private" ? "取消私密" : "设为私密" }}</small>
    </button>

    <button v-if="showDelete" class="action-btn danger-btn" @click="$emit('delete-video')">
      <span class="icon-bubble" aria-hidden="true">
        <svg viewBox="0 0 24 24" class="action-svg">
          <path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z" />
        </svg>
      </span>
      <small class="count-label">删除</small>
    </button>

    <div class="vinyl-wrap" aria-hidden="true">
      <div class="vinyl-disc">
        <img v-if="video.author.avatar" :src="video.author.avatar" alt="vinyl-cover" class="vinyl-cover" />
        <span v-else class="vinyl-dot"></span>
      </div>
      <span class="floating-note note-alpha">♪</span>
      <span class="floating-note note-beta">♫</span>
    </div>
  </aside>
</template>

<script setup lang="ts">
import type { Video } from "@/types/domain";

withDefaults(
  defineProps<{
    video: Video;
    showFollow?: boolean;
    showDelete?: boolean;
    // showPrivate 为 true 时显示设为私密与取消私密，只在自己的作品里出现
    showPrivate?: boolean;
  }>(),
  {
    showFollow: true,
    showDelete: false,
    showPrivate: false
  }
);

defineEmits<{
  (e: "toggle-like"): void;
  (e: "toggle-favorite"): void;
  (e: "comment"): void;
  (e: "toggle-follow"): void;
  (e: "share"): void;
  (e: "delete-video"): void;
  (e: "toggle-private"): void;
  // open-profile 进入作者主页，与 toggle-follow 分开：点头像进主页，点 + 才是关注
  (e: "open-profile"): void;
}>();
</script>

<style scoped>
.sidebar {
  position: absolute;
  right: 12px;
  bottom: 80px;
  z-index: 15;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  user-select: none;
}

button {
  border: none;
  background: transparent;
  color: #ffffff;
  padding: 0;
}

.avatar-wrap {
  position: relative;
  margin-bottom: 8px;
}

.avatar-btn {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  border: 2px solid #ffffff;
  overflow: hidden;
  display: grid;
  place-items: center;
  background-color: #1e1e1e;
  transition: transform 0.18s ease;
}

.avatar-btn:hover {
  transform: scale(1.05);
}

.avatar-btn img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-initial {
  font-size: 18px;
  font-weight: 700;
  color: #ffffff;
}

.follow-badge {
  position: absolute;
  bottom: -7px;
  left: 50%;
  transform: translateX(-50%);
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background-color: var(--tiktok-red);
  color: #ffffff;
  display: grid;
  place-items: center;
  border: 2px solid #000000;
  transition: transform 0.2s ease, background-color 0.2s ease;
}

.follow-badge:hover {
  transform: translateX(-50%) scale(1.12);
}

.follow-badge.followed {
  background-color: rgba(255, 255, 255, 0.3);
}

.plus-icon {
  font-size: 15px;
  font-weight: 900;
  line-height: 1;
  margin-top: -1px;
}

.check-icon {
  font-size: 12px;
  font-weight: 800;
  line-height: 1;
}

.action-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  transition: transform 0.15s ease;
}

.action-btn:hover {
  transform: scale(1.06);
}

.action-btn:active {
  transform: scale(0.92);
}

.icon-bubble {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background-color: rgba(255, 255, 255, 0.12);
  display: grid;
  place-items: center;
  backdrop-filter: blur(8px);
  transition: background-color 0.2s ease;
}

.action-svg {
  width: 24px;
  height: 24px;
  fill: #ffffff;
  transition: fill 0.2s ease, transform 0.2s ease;
}

.count-label {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.95);
  font-variant-numeric: tabular-nums;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.8);
}

.like-btn.active .action-svg {
  fill: var(--tiktok-red);
  animation: heart-beat 0.35s cubic-bezier(0.17, 0.89, 0.32, 1.49);
}

.like-btn.active .icon-bubble {
  background-color: rgba(254, 44, 85, 0.2);
}

.fav-btn.active .action-svg {
  fill: var(--tiktok-yellow);
  animation: fav-glow 0.35s ease;
}

.fav-btn.active .icon-bubble {
  background-color: rgba(250, 206, 21, 0.2);
}

.danger-btn:hover .action-svg {
  fill: var(--tiktok-red);
}

.vinyl-wrap {
  position: relative;
  width: 44px;
  height: 44px;
  margin-top: 8px;
}

.vinyl-disc {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: radial-gradient(circle, #2a2a2a 35%, #0a0a0a 36%, #1c1c1c 65%, #000000 66%);
  box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.2);
  display: grid;
  place-items: center;
  animation: rotate-disc 4s linear infinite;
  overflow: hidden;
}

.vinyl-cover {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  object-fit: cover;
}

.vinyl-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background-color: var(--tiktok-cyan);
}

.floating-note {
  position: absolute;
  top: 6px;
  left: 6px;
  color: #ffffff;
  font-size: 14px;
  opacity: 0;
  pointer-events: none;
}

.note-alpha {
  animation: float-note-one 2.8s ease-in-out infinite;
}

.note-beta {
  animation: float-note-two 2.8s ease-in-out infinite 1.4s;
}

@keyframes rotate-disc {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@keyframes heart-beat {
  0% {
    transform: scale(1);
  }
  45% {
    transform: scale(1.35);
  }
  75% {
    transform: scale(0.92);
  }
  100% {
    transform: scale(1);
  }
}

@keyframes fav-glow {
  0% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.3);
  }
  100% {
    transform: scale(1);
  }
}

@keyframes float-note-one {
  0% {
    opacity: 0;
    transform: translate(0, 0) scale(0.5);
  }
  40% {
    opacity: 0.9;
  }
  100% {
    opacity: 0;
    transform: translate(-26px, -45px) scale(1.1) rotate(-30deg);
  }
}

@keyframes float-note-two {
  0% {
    opacity: 0;
    transform: translate(0, 0) scale(0.5);
  }
  40% {
    opacity: 0.9;
  }
  100% {
    opacity: 0;
    transform: translate(-30px, -55px) scale(1.2) rotate(25deg);
  }
}
</style>
