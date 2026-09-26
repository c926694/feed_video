<template>
  <article class="video-card" :class="{ framed }">
    <video
      ref="videoRef"
      class="video-player"
      :poster="video.coverUrl"
      :src="video.playUrl"
      :muted="effectiveMuted"
      loop
      playsinline
      preload="metadata"
      :autoplay="active"
      @click="onVideoTap"
      @timeupdate="onTimeUpdate"
      @durationchange="onDurationChange"
      @loadedmetadata="onLoadedMetadata"
    />

    <button class="overlay sound-btn" @click.stop="toggleMute">
      {{ isMuted ? "开声" : "静音" }}
    </button>

    <ActionSidebar
      :video="video"
      :show-follow="showFollow"
      :show-delete="showDelete"
      @toggle-like="$emit('toggle-like')"
      @comment="$emit('open-comment')"
      @toggle-follow="$emit('toggle-follow')"
      @share="$emit('share')"
      @delete-video="$emit('delete-video')"
    />

    <div class="overlay info">
      <div class="author-row">
        <img v-if="video.author.avatar" :src="video.author.avatar" alt="作者头像" class="author-avatar" />
        <span v-else class="author-fallback">{{ (video.author.nickname || video.author.username || "匿").slice(0, 1) }}</span>
        <h3>作者：{{ video.author.nickname || video.author.username }}</h3>
      </div>
      <p class="title">{{ video.title }}</p>
      <p class="desc">{{ video.description }}</p>
    </div>

    <div class="overlay progress-wrap" @click.stop>
      <span class="time-label">{{ formattedCurrentTime }} / {{ formattedDuration }}</span>
      <input
        class="progress-slider"
        type="range"
        min="0"
        max="100"
        step="0.1"
        :value="progressValue"
        :disabled="duration <= 0"
        @mousedown="onSeekStart"
        @touchstart="onSeekStart"
        @input="onSeekInput"
        @change="onSeekCommit"
      />
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import ActionSidebar from "@/components/feed/ActionSidebar.vue";
import type { Video } from "@/types/domain";

const props = withDefaults(
  defineProps<{
    video: Video;
    active: boolean;
    showFollow?: boolean;
    showDelete?: boolean;
    framed?: boolean;
  }>(),
  {
    showFollow: true,
    showDelete: false,
    framed: false
  }
);

defineEmits<{
  (e: "toggle-like"): void;
  (e: "toggle-follow"): void;
  (e: "open-comment"): void;
  (e: "share"): void;
  (e: "delete-video"): void;
}>();

const videoRef = ref<HTMLVideoElement | null>(null);
const isMuted = ref(false);
const currentTime = ref(0);
const duration = ref(0);
const seeking = ref(false);
const effectiveMuted = computed(() => !props.active || isMuted.value);
const progressValue = computed(() => {
  if (!duration.value) return 0;
  return Math.min(100, Math.max(0, (currentTime.value / duration.value) * 100));
});
const formattedCurrentTime = computed(() => formatTime(currentTime.value));
const formattedDuration = computed(() => formatTime(duration.value));

watch(
  () => props.active,
  async (active) => {
    if (!videoRef.value) return;
    if (active) {
      videoRef.value.muted = isMuted.value;
      videoRef.value.volume = isMuted.value ? 0 : 1;
      try {
        await videoRef.value.play();
      } catch {
        isMuted.value = true;
        videoRef.value.muted = true;
        videoRef.value.volume = 0;
        void videoRef.value.play();
      }
      return;
    }
    videoRef.value.pause();
    videoRef.value.muted = true;
    videoRef.value.volume = 0;
  },
  { immediate: true }
);

function onVideoTap() {
  if (!videoRef.value || !props.active) return;
  if (isMuted.value) {
    isMuted.value = false;
    videoRef.value.muted = false;
    videoRef.value.volume = 1;
    void videoRef.value.play();
    return;
  }
  if (videoRef.value.paused) {
    void videoRef.value.play();
  } else {
    videoRef.value.pause();
  }
}

function toggleMute() {
  if (!videoRef.value || !props.active) return;
  isMuted.value = !isMuted.value;
  videoRef.value.muted = isMuted.value;
  videoRef.value.volume = isMuted.value ? 0 : 1;
  if (!isMuted.value) {
    void videoRef.value.play();
  }
}

function onLoadedMetadata() {
  if (!videoRef.value) return;
  videoRef.value.muted = effectiveMuted.value;
  videoRef.value.volume = effectiveMuted.value ? 0 : 1;
  duration.value = Number.isFinite(videoRef.value.duration) ? videoRef.value.duration : 0;
  currentTime.value = Number.isFinite(videoRef.value.currentTime) ? videoRef.value.currentTime : 0;
}

function onDurationChange() {
  if (!videoRef.value) return;
  duration.value = Number.isFinite(videoRef.value.duration) ? videoRef.value.duration : 0;
}

function onTimeUpdate() {
  if (!videoRef.value || seeking.value) return;
  currentTime.value = Number.isFinite(videoRef.value.currentTime) ? videoRef.value.currentTime : 0;
}

function onSeekStart() {
  seeking.value = true;
}

function onSeekInput(event: Event) {
  if (!videoRef.value) return;
  const target = event.target as HTMLInputElement;
  const percent = Number(target.value);
  if (!Number.isFinite(percent) || duration.value <= 0) return;
  const nextTime = (percent / 100) * duration.value;
  currentTime.value = nextTime;
  videoRef.value.currentTime = nextTime;
}

function onSeekCommit() {
  seeking.value = false;
}

function formatTime(rawSeconds: number) {
  if (!Number.isFinite(rawSeconds) || rawSeconds < 0) return "00:00";
  const totalSeconds = Math.floor(rawSeconds);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}
</script>

<style scoped>
.video-card {
  position: relative;
  width: 100%;
  height: 100svh;
  scroll-snap-align: start;
  overflow: hidden;
  background: #000;
}

.video-card::before {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 44%;
  z-index: 4;
  pointer-events: none;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0), rgba(0, 0, 0, 0.62));
}

.video-card::after {
  content: "";
  position: absolute;
  top: 0;
  right: 0;
  width: 180px;
  height: 100%;
  z-index: 4;
  pointer-events: none;
  background: linear-gradient(270deg, rgba(0, 0, 0, 0.42), rgba(0, 0, 0, 0));
}

.video-card.framed {
  width: calc(100% - 24px);
  max-width: 1180px;
  height: calc(100svh - 86px);
  margin: 8px auto 10px;
  border-radius: 24px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.34);
  background: #07090f;
}

.video-player {
  width: 100%;
  height: 100%;
  /* 竖屏视频放进宽卡片时，contain 会完整显示整幅画面，上下或左右留黑边；
     cover 会按容器比例裁掉多余部分，画面会缺一块 */
  object-fit: contain;
  border-radius: inherit;
}

.overlay {
  position: absolute;
  left: 0;
  right: 0;
}

.sound-btn {
  top: 16px;
  right: 14px;
  left: auto;
  z-index: 9;
  border: 1px solid rgba(255, 255, 255, 0.35);
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 12px;
  color: #fff;
  background: rgba(0, 0, 0, 0.54);
  backdrop-filter: blur(6px);
}

.info {
  left: 14px;
  right: 104px;
  bottom: 84px;
  z-index: 9;
  padding: 10px 12px;
  border-radius: 14px;
  background: rgba(0, 0, 0, 0.34);
  backdrop-filter: blur(4px);
}

h3 {
  margin: 0;
  font-size: 17px;
}

.author-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.author-avatar,
.author-fallback {
  width: 30px;
  height: 30px;
  border-radius: 50%;
}

.author-avatar {
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.45);
}

.author-fallback {
  display: grid;
  place-items: center;
  font-size: 13px;
  background: rgba(255, 255, 255, 0.18);
  color: #fff;
}

.title,
.desc {
  margin: 8px 0 0;
  font-size: 14px;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.55);
}

.desc {
  color: rgba(255, 255, 255, 0.8);
}

.progress-wrap {
  left: 14px;
  right: 14px;
  bottom: 14px;
  z-index: 10;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.38);
  backdrop-filter: blur(4px);
}

.time-label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.85);
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
}

.progress-slider {
  -webkit-appearance: none;
  appearance: none;
  width: 100%;
  height: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.45);
  cursor: pointer;
}

.progress-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #fff;
  border: none;
}

.progress-slider::-moz-range-thumb {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #fff;
  border: none;
}

@media (max-width: 900px) {
  .video-card.framed {
    width: calc(100% - 24px);
    height: calc(100svh - 66px);
    margin: 8px auto;
    border-radius: 22px;
  }
}
</style>
