<template>
  <section class="feed-page">
    <aside class="left-rail">
      <div class="brand">
        <div class="brand-glitch" aria-hidden="true">
          <span class="glitch-cyan"></span>
          <span class="glitch-red"></span>
          <span class="glitch-main"></span>
        </div>
        <strong class="brand-title">Feed</strong>
      </div>

      <nav class="rail-nav">
        <RouterLink class="rail-link" :class="{ active: tab === 'recommend' }" to="/feed" @click="changeTab('recommend')">
          <span class="rail-icon">
            <svg viewBox="0 0 24 24" class="svg-icon">
              <path d="M12 3 3 10.5V21h6v-6h6v6h6V10.5L12 3Z" fill="currentColor" />
            </svg>
          </span>
          <span class="rail-text">推荐</span>
        </RouterLink>

        <RouterLink class="rail-link" :class="{ active: tab === 'follow' }" to="/feed" @click="changeTab('follow')">
          <span class="rail-icon">
            <svg viewBox="0 0 24 24" class="svg-icon">
              <path d="M16 11c1.66 0 3-1.34 3-3s-1.34-3-3-3-3 1.34-3 3 1.34 3 3 3Zm-8 0c1.66 0 3-1.34 3-3S9.66 5 8 5 5 6.34 5 8s1.34 3 3 3Zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5Zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5Z" fill="currentColor" />
            </svg>
          </span>
          <span class="rail-text">关注</span>
        </RouterLink>

        <RouterLink class="rail-link" to="/messages">
          <span class="rail-icon">
            <svg viewBox="0 0 24 24" class="svg-icon">
              <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2Zm8 7L4 6v2l8 5 8-5V6l-8 5Z" fill="currentColor" />
            </svg>
            <span v-if="unreadCount > 0" class="rail-badge">{{ badgeText }}</span>
          </span>
          <span class="rail-text">消息</span>
        </RouterLink>

        <RouterLink class="rail-link" to="/upload">
          <span class="rail-icon">
            <svg viewBox="0 0 24 24" class="svg-icon">
              <path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2Z" fill="currentColor" />
            </svg>
          </span>
          <span class="rail-text">发布</span>
        </RouterLink>

        <RouterLink class="rail-link" to="/profile">
          <span class="rail-icon">
            <svg viewBox="0 0 24 24" class="svg-icon">
              <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4Zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4Z" fill="currentColor" />
            </svg>
          </span>
          <span class="rail-text">我的</span>
        </RouterLink>
      </nav>
    </aside>

    <section class="main-stage">
      <header class="top-bar">
        <div class="feed-tabs">
          <button :class="{ active: tab === 'follow' }" @click="changeTab('follow')">关注</button>
          <span class="tab-divider">|</span>
          <button :class="{ active: tab === 'recommend' }" @click="changeTab('recommend')">推荐</button>
          <span class="tab-divider">|</span>
          <button :class="{ active: tab === 'hot' }" @click="changeTab('hot')">热门</button>
          <button v-if="tab === 'hot' && hotPlayMode" class="playback-back" @click="backToHotBoard">返回热榜</button>
        </div>

        <div class="quick-nav">
          <RouterLink class="upload-btn" to="/upload">
            <span class="plus-icon">+</span>
            <span>上传</span>
          </RouterLink>
          <RouterLink class="profile-link" to="/profile">我的</RouterLink>
        </div>
      </header>

      <div class="stage-body">
        <HotRankBoard
          v-if="hasLoadedHot"
          v-show="tab === 'hot' && !hotPlayMode"
          @play="openHotPlayback"
        />
        <VideoFeed
          v-if="tab === 'hot' && hotPlayMode"
          :tab="tab"
          :initial-hot-videos="hotSeedVideos"
          :initial-hot-video-id="hotSeedVideoId"
        />
        <VideoFeed v-else-if="tab !== 'hot'" :tab="tab" />
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import HotRankBoard from "@/components/feed/HotRankBoard.vue";
import VideoFeed from "@/components/feed/VideoFeed.vue";
import { useUnreadMessages } from "@/composables/useUnreadMessages";
import type { Video } from "@/types/domain";

const tab = ref<"recommend" | "follow" | "hot">("recommend");
const hotPlayMode = ref(false);
const hasLoadedHot = ref(false);
const hotSeedVideos = ref<Video[]>([]);
const hotSeedVideoId = ref(0);

const { unreadCount } = useUnreadMessages();
const badgeText = computed(() => (unreadCount.value > 99 ? "99+" : String(unreadCount.value)));

function changeTab(nextTab: "recommend" | "follow" | "hot") {
  tab.value = nextTab;
  if (nextTab === "hot") {
    hasLoadedHot.value = true;
  }
}

function openHotPlayback(payload: { videoId: number; videos: Video[] }) {
  hotSeedVideoId.value = payload.videoId;
  hotSeedVideos.value = payload.videos.slice();
  hotPlayMode.value = true;
}

function backToHotBoard() {
  hotPlayMode.value = false;
}

watch(
  () => tab.value,
  (value) => {
    if (value === "hot") {
      hasLoadedHot.value = true;
    }
  },
  { immediate: true }
);
</script>

<style scoped>
.feed-page {
  width: 100vw;
  height: 100svh;
  background-color: #000000;
  display: grid;
  grid-template-columns: 220px 1fr;
  overflow: hidden;
}

.left-rail {
  background-color: #000000;
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  padding: 20px 14px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 10px;
  user-select: none;
}

.brand-glitch {
  position: relative;
  width: 28px;
  height: 28px;
}

.glitch-cyan,
.glitch-red,
.glitch-main {
  position: absolute;
  inset: 0;
  border-radius: 6px;
}

.glitch-cyan {
  background: var(--tiktok-cyan);
  transform: translate(-2px, -1px);
  opacity: 0.85;
}

.glitch-red {
  background: var(--tiktok-red);
  transform: translate(2px, 1px);
  mix-blend-mode: screen;
  opacity: 0.85;
}

.glitch-main {
  background: #ffffff;
  clip-path: polygon(25% 10%, 75% 10%, 75% 90%, 25% 90%);
}

.brand-title {
  font-size: 22px;
  font-weight: 800;
  letter-spacing: -0.5px;
  color: #ffffff;
}

.rail-nav {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.rail-link {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 14px;
  border-radius: 10px;
  color: rgba(255, 255, 255, 0.8);
  font-size: 16px;
  font-weight: 600;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.rail-link:hover {
  background-color: rgba(255, 255, 255, 0.06);
  color: #ffffff;
}

.rail-link.active {
  color: var(--tiktok-red);
}

.rail-icon {
  position: relative;
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
}

.rail-badge {
  position: absolute;
  top: -6px;
  right: -8px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 999px;
  background-color: var(--tiktok-red);
  color: #ffffff;
  font-size: 10px;
  font-weight: 700;
  line-height: 16px;
  text-align: center;
}

.svg-icon {
  width: 100%;
  height: 100%;
}

.main-stage {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100svh;
  background-color: #000000;
  min-width: 0;
}

.top-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 58px;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.65) 0%, transparent 100%);
  pointer-events: none;
}

.feed-tabs,
.quick-nav {
  pointer-events: auto;
}

.feed-tabs {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 12px;
}

.feed-tabs button {
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.6);
  font-size: 17px;
  font-weight: 700;
  padding: 6px 4px;
  position: relative;
  transition: color 0.2s ease;
}

.feed-tabs button:hover {
  color: #ffffff;
}

.feed-tabs button.active {
  color: #ffffff;
}

.feed-tabs button.active::after {
  content: "";
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 24px;
  height: 3px;
  border-radius: 2px;
  background-color: #ffffff;
}

.tab-divider {
  color: rgba(255, 255, 255, 0.2);
  font-size: 14px;
}

.playback-back {
  border: none;
  border-radius: 999px;
  padding: 6px 12px;
  background-color: rgba(255, 255, 255, 0.15);
  color: #ffffff;
  font-size: 13px;
  font-weight: 600;
}

.quick-nav {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 14px;
}

.upload-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 16px;
  border-radius: 4px;
  background-color: rgba(255, 255, 255, 0.1);
  color: #ffffff;
  font-size: 14px;
  font-weight: 600;
  transition: background-color 0.15s ease;
}

.upload-btn:hover {
  background-color: rgba(255, 255, 255, 0.16);
}

.plus-icon {
  font-size: 16px;
  line-height: 1;
}

.profile-link {
  color: rgba(255, 255, 255, 0.85);
  font-size: 14px;
  font-weight: 600;
}

.stage-body {
  flex: 1;
  height: 100%;
  overflow: hidden;
}

@media (max-width: 900px) {
  .feed-page {
    grid-template-columns: 1fr;
  }

  .left-rail {
    display: none;
  }

  .top-bar {
    padding: 0 16px;
  }

  .quick-nav {
    display: none;
  }
}
</style>
