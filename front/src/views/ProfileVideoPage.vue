<template>
  <section class="profile-video-page">
    <button class="back-btn" @click="goBack">←</button>

    <main ref="containerRef" class="feed-container" @scroll.passive="onScroll">
      <VideoCard
        v-for="(video, idx) in videos"
        :key="`${video.id}-${idx}`"
        :video="video"
        :active="idx === activeIndex"
        :in-window="Math.abs(idx - activeIndex) <= 1"
        :framed="true"
        :show-follow="isAuthorMode || source !== 'works'"
        :show-delete="!isAuthorMode && source === 'works'"
        :show-private="!isAuthorMode && source === 'works'"
        @toggle-like="toggleLike(video.id)"
        @toggle-favorite="toggleFavorite(video.id)"
        @toggle-follow="toggleFollow(video.id)"
        @open-comment="openComment(video.id)"
        @share="shareVideo(video)"
        @delete-video="onDeleteVideo(video.id)"
        @toggle-private="onTogglePrivate(video)"
        @open-profile="openProfile(video.author.id)"
      />

      <div v-if="loading" class="loading">加载中...</div>
      <div v-else-if="!videos.length" class="loading">{{ emptyText }}</div>

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
import { computed, nextTick, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  deleteVideo,
  fetchAllVideoPages,
  fetchAuthorVideos,
  fetchMyFavorites,
  fetchMyLikes,
  fetchMyVideos,
  setFollow,
  setVideoFavorite,
  setVideoLike,
  setVideoPrivate
} from "@/api";
import CommentDrawer from "@/components/feed/CommentDrawer.vue";
import VideoCard from "@/components/feed/VideoCard.vue";
import { useToast } from "@/composables/useToast";
import type { Video } from "@/types/domain";

type Source = "works" | "favorites" | "likes";

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
const source = parseSource(route.query.source);
// 作者模式：路由带别人主页的 ID，播放他的公开视频，像自己的作品一样能上下刷
const authorId = computed(() => Number(route.params.id ?? 0));
const isAuthorMode = computed(() => authorId.value > 0);
const authorCursor = ref<{ lastCreatedAt: number; lastId: number } | null>(null);
const authorHasMore = ref(false);
const authorPageSize = 20;

const emptyText = computed(() => {
  if (isAuthorMode.value) return "TA 还没有发布视频";
  if (source === "favorites") return "还没有收藏的视频";
  if (source === "likes") return "还没有点赞的视频";
  return "暂无视频";
});

function parseSource(raw: unknown): Source {
  if (raw === "favorites" || raw === "likes") return raw;
  return "works";
}

function loadVideos(): Promise<Video[]> {
  if (source === "favorites") return fetchAllVideoPages(fetchMyFavorites);
  if (source === "likes") return fetchAllVideoPages(fetchMyLikes);
  return fetchMyVideos(120);
}

// loadAuthorVideos 取作者的一页视频，并记住下一页的游标
async function loadAuthorVideos() {
  const page = await fetchAuthorVideos(authorId.value, { limit: authorPageSize });
  videos.value = page.videos;
  authorCursor.value = page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null;
  authorHasMore.value = page.hasMore;
}

async function loadMoreAuthorVideos() {
  if (!isAuthorMode.value || !authorCursor.value || !authorHasMore.value) return;
  const page = await fetchAuthorVideos(authorId.value, {
    limit: authorPageSize,
    lastCreatedAt: authorCursor.value.lastCreatedAt,
    lastId: authorCursor.value.lastId
  });
  videos.value = [...videos.value, ...page.videos];
  authorCursor.value = page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null;
  authorHasMore.value = page.hasMore;
}

async function bootstrap() {
  loading.value = true;
  try {
    if (isAuthorMode.value) {
      await loadAuthorVideos();
    } else {
      videos.value = await loadVideos();
    }
    if (!videos.value.length) return;

    let idx = videos.value.findIndex((item) => item.id === targetVideoId);
    if (idx < 0) idx = 0;
    activeIndex.value = idx;

    await nextTick();
    // 首屏瞬时定位，点哪条就停在哪条，不播放滚动过程
    scrollToActive("auto");
  } finally {
    loading.value = false;
  }
}

function goBack() {
  if (isAuthorMode.value) {
    router.push(`/profile/${authorId.value}`);
    return;
  }
  router.push({ path: "/profile", query: { tab: source } });
}

// openProfile 进入作者主页
function openProfile(authorId: number) {
  if (!authorId) return;
  router.push(`/profile/${authorId}`);
}

function onScroll() {
  const node = containerRef.value;
  if (!node) return;
  const nextIndex = Math.round(node.scrollTop / Math.max(1, node.clientHeight));
  activeIndex.value = Math.max(0, Math.min(nextIndex, videos.value.length - 1));
  // 刷到最后一条时把作者的下一页接上
  if (isAuthorMode.value && activeIndex.value >= videos.value.length - 1) {
    void loadMoreAuthorVideos();
  }
}

function openComment(videoId: number) {
  commentVideoId.value = videoId;
  commentOpen.value = true;
}

async function toggleLike(videoId: number) {
  const current = videos.value.find((item) => item.id === videoId);
  if (!current) return;
  const targetLiked = await setVideoLike(videoId, !current.liked);
  // 点赞列表里取消点赞后这条不再属于该列表，直接移出
  if (source === "likes" && !targetLiked) {
    await removeFromList(videoId, "已取消点赞");
    return;
  }
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

async function toggleFavorite(videoId: number) {
  const current = videos.value.find((item) => item.id === videoId);
  if (!current) return;
  const targetFavorited = await setVideoFavorite(videoId, !current.favorited);
  // 收藏列表里取消收藏后这条不再属于该列表，直接移出
  if (source === "favorites" && !targetFavorited) {
    await removeFromList(videoId, "已取消收藏");
    return;
  }
  videos.value = videos.value.map((item) => {
    if (item.id !== videoId) return item;
    const delta = (targetFavorited ? 1 : 0) - (item.favorited ? 1 : 0);
    return {
      ...item,
      favorited: targetFavorited,
      favoriteCount: Math.max(0, item.favoriteCount + delta)
    };
  });
}

async function toggleFollow(videoId: number) {
  const current = videos.value.find((item) => item.id === videoId);
  if (!current) return;
  const targetFollowed = await setFollow(current.author.id, !current.followed);
  videos.value = videos.value.map((item) => {
    if (item.id !== videoId) return item;
    return {
      ...item,
      followed: targetFollowed
    };
  });
}

// onTogglePrivate 在公开与私密之间切换自己的作品，切换成功后改写本地状态
async function onTogglePrivate(video: Video) {
  const isPrivate = video.status === "private";
  try {
    await setVideoPrivate(video.id, !isPrivate);
    videos.value = videos.value.map((item) =>
      item.id === video.id ? { ...item, status: isPrivate ? "published" : "private" } : item
    );
    showToast(isPrivate ? "已恢复公开" : "已设为私密，只有你自己能看");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  }
}

async function onDeleteVideo(videoId: number) {
  if (!videoId || deletingVideoId.value === videoId) return;
  const confirmed = window.confirm("确定删除这个视频吗？");
  if (!confirmed) return;

  deletingVideoId.value = videoId;
  try {
    await deleteVideo(videoId);
    await removeFromList(videoId, "视频已删除");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    deletingVideoId.value = 0;
  }
}

// removeFromList 把一条视频移出当前列表，并让相邻的一条顶上来
async function removeFromList(videoId: number, toastText: string) {
  const removedIndex = videos.value.findIndex((item) => item.id === videoId);
  videos.value = videos.value.filter((item) => item.id !== videoId);

  if (!videos.value.length) {
    showToast(toastText);
    goBack();
    return;
  }

  const fallbackIndex = removedIndex >= 0 ? removedIndex : activeIndex.value;
  activeIndex.value = Math.max(0, Math.min(fallbackIndex, videos.value.length - 1));

  await nextTick();
  scrollToActive();
  showToast(toastText);
}

// scrollToActive 定位到当前这条。进入页面时用 auto，直接落在目标视频上，
// 不播放下滑过程；点上下按钮切换时用 smooth，让用户看到翻页方向
function scrollToActive(behavior: ScrollBehavior = "smooth") {
  const node = containerRef.value;
  if (!node) return;
  node.scrollTo({ top: activeIndex.value * node.clientHeight, behavior });
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
