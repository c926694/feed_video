<template>
  <section class="hot-board">
    <div class="hero-shell">
      <div class="hero-copy">
        <span class="eyebrow">HOT</span>
        <h2>热榜</h2>
        <p>按热度排序，集中查看当前最受欢迎的视频。</p>
      </div>

      <div class="toolbar">
        <label class="search-box">
          <input v-model.trim="keyword" type="search" placeholder="搜索标题 / 作者（本地筛选）" />
        </label>
        <button class="ghost-btn" @click="refresh">刷新</button>
        <button class="solid-btn" :disabled="loading || !hasMore" @click="loadMore">
          {{ loading && !videos.length ? "加载中..." : "加载更多" }}
        </button>
      </div>
    </div>

    <div class="board-shell">
      <div v-if="loading && !videos.length" class="empty-state">热榜加载中...</div>
      <div v-else-if="!filteredVideos.length" class="empty-state">没有匹配的视频</div>

      <article v-for="(video, idx) in filteredVideos" :key="video.id" class="rank-card">
        <div class="rank-badge" :class="badgeClass(idx)">{{ idx + 1 }}</div>

        <button class="cover-wrap" @click="openVideo(video)">
          <img v-if="video.coverUrl" :src="video.coverUrl" :alt="video.title" class="cover-image" />
          <div v-else class="cover-fallback">暂无封面</div>
        </button>

        <div class="card-main">
          <div class="meta-row">
            <div class="author-line">
              <img v-if="video.author.avatar" :src="video.author.avatar" alt="作者头像" class="author-avatar" />
              <span v-else class="author-fallback">{{ (video.author.nickname || video.author.username || "匿").slice(0, 1) }}</span>
              <div>
                <h3>{{ video.title }}</h3>
                <p class="subline">
                  作者：{{ video.author.nickname || video.author.username }}
                  <span v-if="video.createdAt"> · 创建时间：{{ formatTime(video.createdAt) }}</span>
                </p>
              </div>
            </div>

            <button class="like-pill" :class="{ active: video.liked }" @click="toggleLike(video.id)">
              <span>❤️</span>
              <small>{{ video.likeCount }}</small>
            </button>
            <button class="like-pill" :class="{ active: video.favorited }" @click="toggleFavorite(video.id)">
              <span>⭐</span>
              <small>{{ video.favoriteCount }}</small>
            </button>
          </div>

          <p class="summary">{{ video.description || "这个视频暂时还没有描述。" }}</p>

          <div class="actions">
            <button class="action-btn" @click="openVideo(video)">进入播放</button>
            <button class="action-btn" @click="openComment(video.id)">查看评论</button>
          </div>
        </div>
      </article>

      <div v-if="!loading && videos.length > 0 && !hasMore" class="end-tip">已经到底了~</div>
    </div>

    <CommentDrawer :open="commentOpen" :video-id="commentVideoId" @close="commentOpen = false" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { fetchHotVideos, setVideoFavorite, setVideoLike } from "@/api";
import CommentDrawer from "@/components/feed/CommentDrawer.vue";
import { useToast } from "@/composables/useToast";
import type { Video } from "@/types/domain";
import { dedupeById } from "@/utils/collections";

const { showToast } = useToast();

const loading = ref(false);
const keyword = ref("");
const videos = ref<Video[]>([]);
const nextOffset = ref(0);
const hasMore = ref(true);
const hotInterval = ref(60);
const commentOpen = ref(false);
const commentVideoId = ref(0);

const emit = defineEmits<{
  (e: "play", payload: { videoId: number; videos: Video[] }): void;
}>();

const filteredVideos = computed(() => {
  if (!keyword.value) return videos.value;
  const query = keyword.value.toLowerCase();
  return videos.value.filter((video) => {
    const authorName = (video.author.nickname || video.author.username).toLowerCase();
    return (
      video.title.toLowerCase().includes(query) ||
      video.description.toLowerCase().includes(query) ||
      authorName.includes(query)
    );
  });
});

async function refresh() {
  loading.value = true;
  try {
    const result = await fetchHotVideos({
      interval: hotInterval.value,
      offset: 0,
      limit: 5
    });
    videos.value = result.videos;
    nextOffset.value = result.nextOffset;
    hasMore.value = result.hasMore;
  } finally {
    loading.value = false;
  }
}

async function loadMore() {
  if (loading.value || !hasMore.value) return;
  loading.value = true;
  try {
    const result = await fetchHotVideos({
      interval: hotInterval.value,
      offset: nextOffset.value,
      limit: 5
    });
    videos.value = dedupeById(videos.value.concat(result.videos));
    nextOffset.value = result.nextOffset;
    hasMore.value = result.hasMore;
  } finally {
    loading.value = false;
  }
}

async function toggleLike(videoId: number) {
  const current = videos.value.find((item) => item.id === videoId);
  if (!current) return;
  const targetLiked = await setVideoLike(videoId, !current.liked);
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

function openComment(videoId: number) {
  commentVideoId.value = videoId;
  commentOpen.value = true;
}

function openVideo(video: Video) {
  if (!video.playUrl) {
    showToast("视频地址不可用");
    return;
  }
  emit("play", {
    videoId: video.id,
    videos: videos.value
  });
}

function formatTime(value: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return `${date.getFullYear()}/${String(date.getMonth() + 1).padStart(2, "0")}/${String(date.getDate()).padStart(2, "0")} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function badgeClass(idx: number) {
  if (idx === 0) return "top-1";
  if (idx === 1) return "top-2";
  if (idx === 2) return "top-3";
  return "top-rest";
}

onMounted(() => {
  void refresh();
});
</script>

<style scoped>
.hot-board {
  height: 100svh;
  overflow-y: auto;
  padding-top: 64px;
  background-color: #000000;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.hot-board::-webkit-scrollbar {
  display: none;
}

.hero-shell {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  padding: 22px 24px 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
}

.eyebrow {
  display: inline-block;
  margin-bottom: 8px;
  font-size: 13px;
  letter-spacing: 0.16em;
  color: rgba(255, 190, 206, 0.7);
}

.hero-copy h2 {
  margin: 0;
  font-size: 34px;
}

.hero-copy p {
  margin: 8px 0 0;
  color: rgba(219, 225, 239, 0.74);
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.search-box {
  width: min(420px, 40vw);
}

.search-box input {
  width: 100%;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.04);
  color: #fff;
  padding: 12px 16px;
}

.ghost-btn,
.solid-btn,
.action-btn,
.like-pill,
.cover-wrap {
  border: none;
  cursor: pointer;
}

.ghost-btn,
.solid-btn,
.action-btn {
  border-radius: 999px;
  padding: 11px 18px;
  color: #fff;
}

.ghost-btn {
  background: rgba(255, 255, 255, 0.06);
}

.solid-btn {
  background: linear-gradient(135deg, #8c2246, #5a6c8f);
}

.board-shell {
  padding: 22px 24px 28px;
}

.rank-card {
  display: grid;
  grid-template-columns: 58px 308px 1fr;
  gap: 16px;
  align-items: stretch;
  padding: 14px;
  margin-bottom: 14px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(12px);
  transition: transform 0.18s ease, border-color 0.18s ease;
}

.rank-card:hover {
  transform: translateY(-2px);
  border-color: rgba(255, 255, 255, 0.14);
}

.rank-badge {
  width: 58px;
  height: 58px;
  border-radius: 20px;
  display: grid;
  place-items: center;
  font-size: 28px;
  font-weight: 700;
  align-self: start;
}

.top-1 {
  background: linear-gradient(180deg, rgba(255, 84, 129, 0.28), rgba(107, 28, 59, 0.4));
  color: #ffdce8;
}

.top-2,
.top-3 {
  background: rgba(140, 44, 78, 0.26);
  color: #f6ceda;
}

.top-rest {
  background: rgba(255, 255, 255, 0.05);
  color: rgba(255, 255, 255, 0.9);
}

.cover-wrap {
  padding: 0;
  border-radius: 20px;
  overflow: hidden;
  background: #10141e;
}

.cover-image {
  width: 100%;
  height: 100%;
  min-height: 188px;
  object-fit: contain;
  background-color: #05070e;
}

.cover-fallback {
  width: 100%;
  height: 100%;
  min-height: 188px;
  display: grid;
  place-items: center;
  color: rgba(255, 255, 255, 0.65);
}

.card-main {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 12px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.author-line {
  display: flex;
  gap: 12px;
  align-items: center;
}

.author-avatar,
.author-fallback {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  flex: 0 0 auto;
}

.author-avatar {
  object-fit: cover;
}

.author-fallback {
  display: grid;
  place-items: center;
  background: rgba(255, 255, 255, 0.12);
}

.author-line h3 {
  margin: 0;
  font-size: 28px;
  line-height: 1.05;
}

.subline {
  margin: 8px 0 0;
  color: rgba(220, 226, 238, 0.66);
}

.summary {
  margin: 0;
  font-size: 15px;
  color: rgba(241, 245, 255, 0.82);
}

.actions {
  display: flex;
  gap: 10px;
}

.action-btn {
  background: rgba(255, 255, 255, 0.08);
}

.like-pill {
  align-self: start;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border-radius: 999px;
  padding: 12px 14px;
  background: rgba(255, 255, 255, 0.05);
  color: rgba(255, 255, 255, 0.9);
}

.like-pill.active {
  background: rgba(162, 37, 76, 0.38);
  color: #ffd1df;
}

.like-pill span {
  font-size: 16px;
}

.empty-state {
  padding: 80px 0;
  text-align: center;
  color: rgba(222, 230, 242, 0.68);
}

.end-tip {
  margin-top: 4px;
  text-align: center;
  color: rgba(222, 230, 242, 0.72);
  font-size: 13px;
}

@media (max-width: 1200px) {
  .hero-shell {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar {
    flex-wrap: wrap;
  }

  .search-box {
    width: 100%;
  }
}

@media (max-width: 900px) {
  .hot-board {
    min-height: calc(100svh - 56px);
    border-radius: 0;
    border-left: none;
    border-right: none;
  }

  .board-shell,
  .hero-shell {
    padding-left: 14px;
    padding-right: 14px;
  }

  .rank-card {
    grid-template-columns: 1fr;
  }

  .rank-badge {
    width: 50px;
    height: 50px;
    font-size: 24px;
  }

  .cover-image,
  .cover-fallback {
    min-height: 170px;
  }

  .meta-row {
    flex-direction: column;
  }

  .author-line h3 {
    font-size: 22px;
  }
}
</style>



