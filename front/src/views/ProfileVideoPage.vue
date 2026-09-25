<template>
  <section class="profile-video-page">
    <button class="back-btn" @click="goBack">←</button>

    <main ref="containerRef" class="feed-container" @scroll.passive="onScroll">
      <VideoCard
        v-for="(video, idx) in videos"
        :key="`${video.id}-${idx}`"
        :video="video"
        :active="idx === activeIndex"
        :show-follow="false"
        :show-delete="true"
        @toggle-like="toggleLike(video.id)"
        @toggle-follow="noopFollow"
        @open-comment="openComment(video.id)"
        @share="shareVideo(video)"
        @delete-video="onDeleteVideo(video.id)"
      />

      <div v-if="loading" class="loading">加载中...</div>
      <div v-else-if="!videos.length" class="loading">暂无视频</div>

      <div v-if="videos.length > 1" class="switch-nav">
        <button class="switch-btn" :disabled="activeIndex <= 0" @click.stop="switchPrev" aria-label="上一条视频">↑</button>
        <button class="switch-btn" :disabled="activeIndex >= videos.length - 1" @click.stop="switchNext" aria-label="下一条视频">
          ↓
        </button>
      </div>

      <div v-if="videos.length > 0 && activeIndex >= videos.length - 1" class="end-tip">已经到底了~</div>
    </main>

    <CommentDrawer :open="commentOpen" :video-id="commentVideoId" @close="commentOpen = false" />
  </section>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { deleteVideo, fetchMyVideos, switchVideoLike } from "@/api";
import CommentDrawer from "@/components/feed/CommentDrawer.vue";
import VideoCard from "@/components/feed/VideoCard.vue";
import { useToast } from "@/composables/useToast";
import type { Video } from "@/types/domain";

const router = useRouter();
const route = useRoute();
const { showToast } = useToast();

const containerRef = ref<HTMLElement | null>(null);
const videos = ref<Video[]>([]);
const loading = ref(false);
const activeIndex = ref(0);
const commentOpen = ref(false);
const commentVideoId = ref(0);
const deletingVideoId = ref(0);

const targetVideoId = Number(route.query.videoId ?? 0);

async function bootstrap() {
  loading.value = true;
  try {
    videos.value = await fetchMyVideos(120);
    if (!videos.value.length) return;

    let idx = videos.value.findIndex((item) => item.id === targetVideoId);
    if (idx < 0) idx = 0;
    activeIndex.value = idx;

    await nextTick();
    scrollToActive();
  } finally {
    loading.value = false;
  }
}

function goBack() {
  if (window.history.length > 1) {
    router.back();
    return;
  }
  router.push("/profile");
}

function onScroll() {
  const node = containerRef.value;
  if (!node) return;
  const nextIndex = Math.round(node.scrollTop / Math.max(1, node.clientHeight));
  activeIndex.value = Math.max(0, Math.min(nextIndex, videos.value.length - 1));
}

function openComment(videoId: number) {
  commentVideoId.value = videoId;
  commentOpen.value = true;
}

async function toggleLike(videoId: number) {
  const targetLiked = await switchVideoLike(videoId);
  videos.value = videos.value.map((item) => {
    if (item.id !== videoId) return item;
    const delta = (targetLiked ? 1 : 0) - (item.liked ? 1 : 0);
    return {
      ...item,
      liked: targetLiked,
      likeCount: Math.max(0, item.likeCount + delta)
    };
  });
}

async function onDeleteVideo(videoId: number) {
  if (!videoId || deletingVideoId.value === videoId) return;
  const confirmed = window.confirm("确定删除这个视频吗？");
  if (!confirmed) return;

  deletingVideoId.value = videoId;
  try {
    await deleteVideo(videoId);

    const deletedIndex = videos.value.findIndex((item) => item.id === videoId);
    videos.value = videos.value.filter((item) => item.id !== videoId);

    if (!videos.value.length) {
      showToast("视频已删除");
      router.push("/profile");
      return;
    }

    const fallbackIndex = deletedIndex >= 0 ? deletedIndex : activeIndex.value;
    activeIndex.value = Math.max(0, Math.min(fallbackIndex, videos.value.length - 1));

    await nextTick();
    scrollToActive();
    showToast("视频已删除");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    deletingVideoId.value = 0;
  }
}

function scrollToActive() {
  const node = containerRef.value;
  if (!node) return;
  node.scrollTo({ top: activeIndex.value * node.clientHeight, behavior: "auto" });
}

function switchPrev() {
  if (activeIndex.value <= 0) return;
  activeIndex.value -= 1;
  scrollToActive();
}

function switchNext() {
  if (activeIndex.value >= videos.value.length - 1) return;
  activeIndex.value += 1;
  scrollToActive();
}

function shareVideo(video: Video) {
  navigator.clipboard.writeText(video.playUrl).catch(() => undefined);
  showToast("已复制视频链接");
}

function noopFollow() {
  return;
}

onMounted(() => {
  void bootstrap();
});
</script>

<style scoped>
.profile-video-page {
  position: relative;
  height: 100svh;
  background: #000;
}

.feed-container {
  height: 100svh;
  overflow-y: auto;
  scroll-snap-type: y mandatory;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.feed-container::-webkit-scrollbar {
  display: none;
}

.back-btn {
  position: fixed;
  top: 14px;
  left: 12px;
  z-index: 20;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.35);
  background: rgba(0, 0, 0, 0.45);
  color: #fff;
  font-size: 22px;
  line-height: 1;
  display: grid;
  place-items: center;
}

.loading {
  position: fixed;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  z-index: 12;
  color: rgba(255, 255, 255, 0.8);
  font-size: 13px;
}

.switch-nav {
  position: fixed;
  right: 18px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 25;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.switch-btn {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.35);
  background: rgba(0, 0, 0, 0.48);
  color: #fff;
  font-size: 18px;
  line-height: 1;
  display: grid;
  place-items: center;
  cursor: pointer;
  backdrop-filter: blur(6px);
}

.switch-btn:disabled {
  cursor: not-allowed;
  opacity: 0.35;
}

.end-tip {
  position: fixed;
  left: 50%;
  bottom: 18px;
  transform: translateX(-50%);
  z-index: 26;
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(0, 0, 0, 0.45);
  color: rgba(255, 255, 255, 0.88);
  font-size: 12px;
  backdrop-filter: blur(4px);
}
</style>
