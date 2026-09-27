<template>
  <section class="grid-wrap">
    <h3>我的视频</h3>
    <div v-if="videos.length" class="grid">
      <div v-for="video in videos" :key="video.id" class="cell">
        <RouterLink
          v-if="video.status === 'published'"
          class="item"
          :to="{ path: '/profile/videos', query: { videoId: video.id } }"
        >
          <img v-if="video.coverUrl" :src="video.coverUrl" :alt="video.title" />
          <div v-else class="fallback">暂无封面</div>
          <footer>
            <strong>{{ video.title }}</strong>
            <small>点赞 {{ video.likeCount }} · 评论 {{ video.commentCount }}</small>
          </footer>
        </RouterLink>

        <div v-else class="item pending" :class="{ failed: isFailed(video.id) }">
          <img v-if="coverOf(video)" :src="coverOf(video)" :alt="video.title" />
          <div v-else class="fallback">暂无封面</div>
          <div class="mask">
            <span class="mask-title">{{ maskTitle(video) }}</span>
            <div class="actions">
              <button v-if="isFailed(video.id)" class="mini-btn" @click="onRetry(video.id)">重试</button>
              <button class="mini-btn danger" @click="onDelete(video.id)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </div>
    <p v-else class="empty">还没有可展示的视频，先去发布一个吧。</p>
  </section>
</template>

<script setup lang="ts">
import { deleteVideo } from "@/api";
import { removeTask, retryTask, useUploadQueue } from "@/composables/useUploadQueue";
import type { Video } from "@/types/domain";

defineProps<{
  videos: Video[];
}>();

const emit = defineEmits<{
  (event: "changed"): void;
}>();

const { tasks } = useUploadQueue();

function taskOf(videoId: number) {
  return tasks.value.find((item) => item.videoId === videoId);
}

function isFailed(videoId: number) {
  return taskOf(videoId)?.status === "failed";
}

function maskTitle(video: Video) {
  if (video.status === "failed" || isFailed(video.id)) return "上传失败";
  const task = taskOf(video.id);
  if (task) return `上传中 ${task.progress}%`;
  return "上传中";
}

function coverOf(video: Video) {
  return taskOf(video.id)?.coverLocalUrl ?? video.coverUrl;
}

async function onDelete(videoId: number) {
  await deleteVideo(videoId);
  removeTask(videoId);
  emit("changed");
}

async function onRetry(videoId: number) {
  await retryTask(videoId);
}
</script>

<style scoped>
.grid-wrap {
  padding: 8px 14px 120px;
  max-width: 760px;
  margin: 0 auto;
}

h3 {
  margin: 0 0 10px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.cell {
  position: relative;
}

.item {
  background: rgba(255, 255, 255, 0.03);
  border-radius: 10px;
  overflow: hidden;
  text-decoration: none;
  color: inherit;
  display: block;
}

img,
.fallback {
  width: 100%;
  aspect-ratio: 3 / 4;
  object-fit: cover;
}

.fallback {
  display: grid;
  place-items: center;
  color: var(--text-muted);
  font-size: 12px;
}

footer {
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

strong {
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

small {
  color: var(--text-muted);
  font-size: 11px;
}

.pending {
  cursor: default;
}

.mask {
  position: absolute;
  inset: 0;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.55);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.failed .mask {
  background: rgba(40, 40, 40, 0.7);
}

.mask-title {
  color: #fff;
  font-size: 12px;
}

.actions {
  display: flex;
  gap: 6px;
}

.mini-btn {
  border: none;
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 11px;
  color: #fff;
  background: #18b6ff;
  cursor: pointer;
}

.mini-btn.danger {
  background: rgba(255, 80, 80, 0.85);
}

.empty {
  color: var(--text-muted);
}

@media (max-width: 520px) {
  .grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
