<template>
  <section class="video-page">
    <button class="back-btn" type="button" @click="goBack">←</button>

    <main class="single-stage">
      <VideoCard
        v-if="video"
        :video="video"
        :active="true"
        :in-window="true"
        :framed="true"
        :show-follow="!isMyVideo"
        :show-private="isMyVideo"
        @toggle-like="toggleLike(video.id)"
        @toggle-favorite="toggleFavorite(video.id)"
        @toggle-follow="toggleFollow(video.author.id)"
        @open-comment="openComment"
        @share="shareVideo(video)"
        @toggle-private="togglePrivate"
        @open-profile="openProfile(video.author.id)"
      />

      <p v-if="video && isMyVideo && video.status === 'private'" class="private-hint">仅自己可见</p>

      <p v-else-if="loading" class="state-text">加载中...</p>
      <p v-else class="state-text">视频不存在或已下架</p>
    </main>

    <CommentDrawer
      :open="commentOpen"
      :video-id="video?.id ?? 0"
      :focus-comment-id="focusCommentId"
      @close="commentOpen = false"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { fetchMe, fetchVideoDetail, setVideoPrivate } from "@/api";
import CommentDrawer from "@/components/feed/CommentDrawer.vue";
import VideoCard from "@/components/feed/VideoCard.vue";
import { useToast } from "@/composables/useToast";
import { useVideoActions } from "@/composables/useVideoActions";
import type { Video } from "@/types/domain";

const route = useRoute();
const router = useRouter();
const { showToast } = useToast();

const videos = ref<Video[]>([]);
const loading = ref(true);
const currentUserId = ref(0);
const commentOpen = ref(false);
const focusCommentId = ref(0);

const video = computed(() => videos.value[0] ?? null);
const { toggleLike, toggleFavorite, toggleFollow } = useVideoActions(() => [videos.value]);

const isMyVideo = computed(() => currentUserId.value > 0 && video.value?.author.id === currentUserId.value);

function openComment() {
  commentOpen.value = true;
}

function shareVideo(item: Video) {
  navigator.clipboard.writeText(item.playUrl).catch(() => undefined);
  showToast("已复制视频链接");
}

// togglePrivate 在自己的视频上切换公开与私密
async function togglePrivate() {
  const current = videos.value[0];
  if (!current) return;
  const isPrivate = current.status === "private";
  try {
    await setVideoPrivate(current.id, !isPrivate);
    videos.value = [{ ...current, status: isPrivate ? "published" : "private" }];
    showToast(isPrivate ? "已恢复公开" : "已设为私密，只有你自己能看");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  }
}

function goBack() {
  if (window.history.length > 1) {
    router.back();
    return;
  }
  router.push("/feed");
}

// openProfile 进入作者主页
function openProfile(authorId: number) {
  if (!authorId) return;
  router.push(`/profile/${authorId}`);
}

// loadVideo 按路由参数取单条视频，带评论参数时直接打开评论抽屉
async function loadVideo() {
  const videoId = Number(route.params.id ?? 0);
  const focus = Number(route.query.comment ?? 0);
  focusCommentId.value = Number.isFinite(focus) && focus > 0 ? focus : 0;
  if (!videoId) {
    loading.value = false;
    return;
  }
  loading.value = true;
  try {
    videos.value = [await fetchVideoDetail(videoId)];
    if (focusCommentId.value > 0) {
      commentOpen.value = true;
    }
  } catch {
    videos.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  try {
    const me = await fetchMe();
    currentUserId.value = me.id;
  } catch {
    currentUserId.value = 0;
  }
  await loadVideo();
});

watch(
  () => route.fullPath,
  async () => {
    await loadVideo();
  }
);
</script>

<style scoped>
.video-page {
  position: relative;
  min-height: 100svh;
  background-color: #000000;
}

.single-stage {
  height: 100svh;
  display: flex;
  align-items: center;
  justify-content: center;
}

.back-btn {
  position: fixed;
  top: 16px;
  left: 16px;
  z-index: 40;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: none;
  background-color: rgba(0, 0, 0, 0.45);
  color: #ffffff;
  font-size: 18px;
  cursor: pointer;
}

.state-text {
  color: rgba(255, 255, 255, 0.6);
  font-size: 14px;
}

/* 自己的私密视频，提示其他人看不到 */
.private-hint {
  position: fixed;
  top: 18px;
  left: 64px;
  z-index: 40;
  margin: 0;
  padding: 6px 12px;
  border-radius: 999px;
  background-color: rgba(0, 0, 0, 0.5);
  color: #ffffff;
  font-size: 12px;
}
</style>
