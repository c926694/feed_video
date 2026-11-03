<template>
  <section class="grid-wrap">
    <div v-if="videos.length" class="grid">
      <div v-for="video in videos" :key="video.id" class="cell">
        <RouterLink v-if="!isPending(video)" class="item" :to="videoLink(video)">
          <img v-if="video.coverUrl" :src="video.coverUrl" :alt="video.title" />
          <div v-else class="fallback">暂无封面</div>
          <span v-if="video.status === 'private'" class="private-badge">私密</span>
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
    <p v-else class="empty">{{ emptyText }}</p>
  </section>
</template>

<script setup lang="ts">
import { deleteVideo } from "@/api";
import { removeTask, retryTask, useUploadQueue } from "@/composables/useUploadQueue";
import type { Video } from "@/types/domain";

const props = withDefaults(
  defineProps<{
    videos: Video[];
    emptyText?: string;
    // manage 为 true 时未发布完成的视频显示上传进度与重试、删除操作
    manage?: boolean;
    // source 决定点开视频后播放页加载哪个列表
    source?: string;
    // ownerId 是别人主页的用户 ID，填了就跳到那个人的滑动播放页
    ownerId?: number;
  }>(),
  {
    emptyText: "还没有可展示的视频，先去发布一个吧。",
    manage: true,
    source: "works",
    ownerId: 0
  }
);

const emit = defineEmits<{
  (event: "changed"): void;
}>();

const { tasks } = useUploadQueue();

// 私密视频不是上传中的视频，这里只有已创建与上传失败两种状态算未完成
function isPending(video: Video) {
  return props.manage && (video.status === "created" || video.status === "failed");
}

// videoLink 自己的作品跳自己的播放页，别人主页上的作品跳按作者的播放页
function videoLink(video: Video) {
  if (props.ownerId > 0) {
    return { path: `/profile/${props.ownerId}/videos`, query: { videoId: video.id } };
  }
  return { path: "/profile/videos", query: { videoId: video.id, source: props.source } };
}

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
  min-height: 120px;
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

/* 只有作者自己可见的视频，在封面上标出来 */
.private-badge {
  position: absolute;
  top: 6px;
  left: 6px;
  padding: 2px 6px;
  border-radius: 4px;
  background-color: rgba(0, 0, 0, 0.66);
  color: #ffffff;
  font-size: 11px;
  line-height: 16px;
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
  margin: 0;
  padding: 18px 2px;
  font-size: 13px;
}
</style>
