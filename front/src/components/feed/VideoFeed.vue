<template>
  <main ref="containerRef" class="feed-container" @scroll.passive="onScroll" @wheel="onWheel">
    <VideoCard
      v-for="(video, idx) in displayVideos"
      :key="`${video.id}-${idx}`"
      :video="video"
      :active="idx === activeIndex"
      :in-window="Math.abs(idx - activeIndex) <= 1"
      :framed="true"
      @toggle-like="toggleLike(video.id)"
      @toggle-favorite="toggleFavorite(video.id)"
      @toggle-follow="toggleFollow(video.author.id)"
      @open-comment="openComment(video.id)"
      @share="shareVideo(video)"
    />

    <div v-if="loading" class="loading">加载中...</div>
    <div v-else-if="!displayVideos.length" class="loading">暂无视频内容</div>

    <CommentDrawer :open="commentOpen" :video-id="commentVideoId" @close="commentOpen = false" />

    <div v-if="displayVideos.length > 1" class="switch-nav">
      <button class="switch-btn" :disabled="activeIndex <= 0" @click.stop="switchPrev" aria-label="上一条视频">↑</button>
      <button
        class="switch-btn"
        :disabled="activeIndex >= displayVideos.length - 1"
        @click.stop="switchNext"
        aria-label="下一条视频"
      >
        ↓
      </button>
    </div>

    <div v-if="showEndTip" class="end-tip">已经到底了~</div>
  </main>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { fetchFeedVideos, fetchFollowVideos, fetchHotVideos, setFollow, setVideoFavorite, setVideoLike } from "@/api";
import CommentDrawer from "@/components/feed/CommentDrawer.vue";
import VideoCard from "@/components/feed/VideoCard.vue";
import type { Video } from "@/types/domain";
import { useToast } from "@/composables/useToast";
import { dedupeById } from "@/utils/collections";

const { showToast } = useToast();
type FeedTab = "recommend" | "follow" | "hot";

const props = withDefaults(
  defineProps<{
    tab?: FeedTab;
    initialHotVideos?: Video[];
    initialHotVideoId?: number;
  }>(),
  {
    tab: "recommend",
    initialHotVideoId: 0
  }
);

const containerRef = ref<HTMLElement | null>(null);
const loading = ref(false);
const recommendVideos = ref<Video[]>([]);
const followVideos = ref<Video[]>([]);
const hotVideos = ref<Video[]>([]);
interface FeedCursor {
  createdAt: number;
  id: number;
}
const recommendCursor = ref<FeedCursor | null>(null);
const followCursor = ref<FeedCursor | null>(null);
const hotNextOffset = ref(0);
const hotHasMore = ref(true);
const hotInterval = 60;
const activeIndex = ref(0);

const commentOpen = ref(false);
const commentVideoId = ref(0);

const displayVideos = computed(() => {
  if (props.tab === "recommend") return recommendVideos.value;
  if (props.tab === "follow") return followVideos.value;
  if (props.tab === "hot") return hotVideos.value;
  return recommendVideos.value;
});
const currentNextScore = computed(() => {
  if (props.tab === "recommend") return recommendCursor.value ? "1" : "";
  if (props.tab === "follow") return followCursor.value ? "1" : "";
  if (props.tab === "hot") return hotHasMore.value ? "1" : "";
  return recommendCursor.value ? "1" : "";
});
const showEndTip = computed(
  () =>
    !loading.value &&
    displayVideos.value.length > 0 &&
    !currentNextScore.value &&
    activeIndex.value >= displayVideos.value.length - 1
);

watch(
  () => displayVideos.value.length,
  async (len) => {
    if (!len) {
      activeIndex.value = 0;
      return;
    }
    if (activeIndex.value >= len) {
      activeIndex.value = len - 1;
    }
    await nextTick();
  }
);

async function loadInitial() {
  loading.value = true;
  try {
    if (props.tab === "hot") {
      if (props.initialHotVideos?.length) {
        hotVideos.value = props.initialHotVideos.slice();
      }
      const hot = await fetchHotVideos({
        interval: hotInterval,
        offset: 0,
        limit: 5
      });
      // 热榜顺序由后端 score 排序决定，前端直接使用返回顺序
      hotVideos.value = hot.videos;
      hotNextOffset.value = hot.nextOffset;
      hotHasMore.value = hot.hasMore;
      await alignHotStartVideo();
      return;
    }

    const feed = await fetchFeedVideos({ limit: 5 });
    recommendVideos.value = feed.videos;
    recommendCursor.value = feed.nextId ? { createdAt: feed.nextCreatedAt, id: feed.nextId } : null;

    if (!recommendVideos.value.length) {
      const hot = await fetchHotVideos({
        interval: hotInterval,
        offset: 0,
        limit: 5
      });
      hotVideos.value = hot.videos;
      hotNextOffset.value = hot.nextOffset;
      hotHasMore.value = hot.hasMore;
    }
  } finally {
    loading.value = false;
  }
}

function upsertVideosKeepOrder(base: Video[], incoming: Video[]) {
  if (!incoming.length) return base;
  const incomingMap = new Map<number, Video>();
  for (const item of incoming) {
    incomingMap.set(item.id, item);
  }

  const seen = new Set<number>();
  const result: Video[] = base.map((item) => {
    seen.add(item.id);
    return incomingMap.get(item.id) ?? item;
  });

  for (const item of incoming) {
    if (!seen.has(item.id)) {
      result.push(item);
    }
  }

  return result;
}

function scrollToIndex(index: number) {
  const node = containerRef.value;
  if (!node) return;
  node.scrollTo({
    top: index * node.clientHeight,
    behavior: "smooth"
  });
}

let isWheeling = false;

function onWheel(event: WheelEvent) {
  if (Math.abs(event.deltaY) < 18) return;
  if (isWheeling) {
    event.preventDefault();
    return;
  }
  if (event.deltaY > 0) {
    if (activeIndex.value < displayVideos.value.length - 1) {
      event.preventDefault();
      isWheeling = true;
      switchNext();
      setTimeout(() => {
        isWheeling = false;
      }, 400);
    }
  } else if (event.deltaY < 0) {
    if (activeIndex.value > 0) {
      event.preventDefault();
      isWheeling = true;
      switchPrev();
      setTimeout(() => {
        isWheeling = false;
      }, 400);
    }
  }
}

async function alignHotStartVideo() {
  if (props.tab !== "hot" || !props.initialHotVideoId) return;
  const targetIndex = hotVideos.value.findIndex((item) => item.id === props.initialHotVideoId);
  if (targetIndex < 0) return;
  activeIndex.value = targetIndex;
  await nextTick();
  scrollToIndex(targetIndex);
}

function switchPrev() {
  if (activeIndex.value <= 0) return;
  const next = activeIndex.value - 1;
  activeIndex.value = next;
  scrollToIndex(next);
}

function switchNext() {
  if (activeIndex.value >= displayVideos.value.length - 1) return;
  const next = activeIndex.value + 1;
  activeIndex.value = next;
  scrollToIndex(next);
}

async function loadMore() {
  if (loading.value) return;
  loading.value = true;
  try {
    if (props.tab === "follow") {
      if (!followCursor.value) return;
      const follow = await fetchFollowVideos({
        limit: 5,
        lastCreatedAt: followCursor.value.createdAt,
        lastId: followCursor.value.id
      });
      followVideos.value = upsertVideosKeepOrder(followVideos.value, follow.videos);
      followCursor.value = follow.nextId ? { createdAt: follow.nextCreatedAt, id: follow.nextId } : null;
      return;
    }
    if (props.tab === "hot") {
      if (!hotHasMore.value) return;
      const hot = await fetchHotVideos({
        interval: hotInterval,
        offset: hotNextOffset.value,
        limit: 5
      });
      // 跨分钟时合并键会重建，相邻两页可能重叠，拼接后按 ID 去重
      hotVideos.value = dedupeById(hotVideos.value.concat(hot.videos));
      hotNextOffset.value = hot.nextOffset;
      hotHasMore.value = hot.hasMore;
      return;
    }
    if (!recommendCursor.value) return;
    const feed = await fetchFeedVideos({
      limit: 5,
      lastCreatedAt: recommendCursor.value.createdAt,
      lastId: recommendCursor.value.id
    });
    recommendVideos.value = upsertVideosKeepOrder(recommendVideos.value, feed.videos);
    recommendCursor.value = feed.nextId ? { createdAt: feed.nextCreatedAt, id: feed.nextId } : null;
  } finally {
    loading.value = false;
  }
}

async function toggleLike(videoId: number) {
  const current =
    recommendVideos.value.find((item) => item.id === videoId) ??
    followVideos.value.find((item) => item.id === videoId) ??
    hotVideos.value.find((item) => item.id === videoId);
  if (!current) return;
  const targetLiked = await setVideoLike(videoId, !current.liked);
  const patch = (videos: Video[]) =>
    videos.map((item) => {
      if (item.id !== videoId) return item;
      const liked = targetLiked;
      const delta = (liked ? 1 : 0) - (item.liked ? 1 : 0);
      return {
        ...item,
        liked,
        likeCount: Math.max(0, item.likeCount + delta)
      };
    });
  recommendVideos.value = patch(recommendVideos.value);
  followVideos.value = patch(followVideos.value);
  hotVideos.value = patch(hotVideos.value);
}

async function toggleFavorite(videoId: number) {
  const current =
    recommendVideos.value.find((item) => item.id === videoId) ??
    followVideos.value.find((item) => item.id === videoId) ??
    hotVideos.value.find((item) => item.id === videoId);
  if (!current) return;
  const targetFavorited = await setVideoFavorite(videoId, !current.favorited);
  const patch = (videos: Video[]) =>
    videos.map((item) => {
      if (item.id !== videoId) return item;
      const favorited = targetFavorited;
      const delta = (favorited ? 1 : 0) - (item.favorited ? 1 : 0);
      return {
        ...item,
        favorited,
        favoriteCount: Math.max(0, item.favoriteCount + delta)
      };
    });
  recommendVideos.value = patch(recommendVideos.value);
  followVideos.value = patch(followVideos.value);
  hotVideos.value = patch(hotVideos.value);
}

async function toggleFollow(userId: number) {
  if (!userId) return;
  const current =
    recommendVideos.value.find((item) => item.author.id === userId) ??
    followVideos.value.find((item) => item.author.id === userId) ??
    hotVideos.value.find((item) => item.author.id === userId);
  if (!current) return;
  const targetFollowed = await setFollow(userId, !current.followed);
  const patchFollow = (videos: Video[]) =>
    videos.map((item) =>
      item.author.id === userId
        ? {
            ...item,
            followed: targetFollowed
          }
        : item
    );
  recommendVideos.value = patchFollow(recommendVideos.value);
  followVideos.value = patchFollow(followVideos.value);
  hotVideos.value = patchFollow(hotVideos.value);

  followCursor.value = null;
  followVideos.value = [];
  if (props.tab === "follow") {
    await loadFollowInitial();
  }
}

async function loadFollowInitial() {
  loading.value = true;
  try {
    const follow = await fetchFollowVideos({ limit: 5 });
    followVideos.value = follow.videos;
    followCursor.value = follow.nextId ? { createdAt: follow.nextCreatedAt, id: follow.nextId } : null;
  } finally {
    loading.value = false;
  }
}

function openComment(videoId: number) {
  commentVideoId.value = videoId;
  commentOpen.value = true;
}

function shareVideo(video: Video) {
  navigator.clipboard.writeText(video.playUrl).catch(() => undefined);
  showToast("已复制视频链接");
}

watch(
  () => props.tab,
  async (tab) => {
    activeIndex.value = 0;
    if (containerRef.value) {
      containerRef.value.scrollTo({ top: 0, behavior: "auto" });
    }
    if (tab === "hot") {
      loading.value = true;
      try {
        const hot = await fetchHotVideos({
          interval: hotInterval,
          offset: 0,
          limit: 5
        });
        hotVideos.value = hot.videos;
        hotNextOffset.value = hot.nextOffset;
        hotHasMore.value = hot.hasMore;
      } finally {
        loading.value = false;
      }
    }
    if (tab === "follow" && !followVideos.value.length) {
      await loadFollowInitial();
    }
  }
);

function onScroll() {
  const node = containerRef.value;
  if (!node) return;

  const nextIndex = Math.round(node.scrollTop / Math.max(1, node.clientHeight));
  if (nextIndex !== activeIndex.value) {
    activeIndex.value = Math.max(0, Math.min(nextIndex, displayVideos.value.length - 1));
  }

  const nearBottom = node.scrollTop + node.clientHeight >= node.scrollHeight - node.clientHeight;
  if (nearBottom) {
    void loadMore();
  }
}

void loadInitial();
</script>

<style scoped>
.feed-container {
  height: 100svh;
  overflow-y: auto;
  scroll-snap-type: y mandatory;
  background-color: #000000;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.feed-container::-webkit-scrollbar {
  display: none;
}

.loading {
  position: fixed;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  z-index: 12;
  color: rgba(255, 255, 255, 0.7);
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.5px;
}

.switch-nav {
  position: fixed;
  right: 24px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 25;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.switch-btn {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.08);
  color: #ffffff;
  font-size: 20px;
  line-height: 1;
  display: grid;
  place-items: center;
  cursor: pointer;
  backdrop-filter: blur(12px);
  transition: transform 0.18s ease, background-color 0.18s ease, border-color 0.18s ease;
}

.switch-btn:hover:not(:disabled) {
  transform: scale(1.08);
  background: rgba(255, 255, 255, 0.18);
  border-color: rgba(255, 255, 255, 0.35);
}

.switch-btn:disabled {
  cursor: not-allowed;
  opacity: 0.25;
}

.end-tip {
  position: fixed;
  left: 50%;
  bottom: 24px;
  transform: translateX(-50%);
  z-index: 26;
  padding: 8px 16px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(18, 18, 18, 0.75);
  color: rgba(255, 255, 255, 0.85);
  font-size: 13px;
  font-weight: 500;
  backdrop-filter: blur(10px);
}
</style>
