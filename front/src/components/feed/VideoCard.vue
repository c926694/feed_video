<template>
  <article class="video-card" :class="{ framed }">
    <div class="ambient-backdrop" aria-hidden="true">
      <img v-if="video.coverUrl" :src="video.coverUrl" alt="ambient" class="ambient-img" />
    </div>

    <div class="player-wrapper">
      <video
        v-if="inWindow"
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
      <img
        v-else
        class="video-player"
        :src="video.coverUrl"
        alt="video-poster"
      />

      <div v-if="isPaused && inWindow" class="play-state-overlay" @click="onVideoTap">
        <span class="big-play-icon">▶</span>
      </div>
    </div>

    <div class="vignette-bottom" aria-hidden="true"></div>

    <button class="sound-pill" :title="isMuted ? '点击开启声音' : '点击静音'" @click.stop="toggleMute">
      <span class="sound-icon">{{ isMuted ? "🔇" : "🔊" }}</span>
      <span class="sound-text">{{ isMuted ? "静音" : "开声" }}</span>
    </button>

    <ActionSidebar
      :video="video"
      :show-follow="showFollow"
      :show-delete="showDelete"
      :show-private="showPrivate"
      @toggle-like="$emit('toggle-like')"
      @toggle-favorite="$emit('toggle-favorite')"
      @comment="$emit('open-comment')"
      @toggle-follow="$emit('toggle-follow')"
      @share="$emit('share')"
      @delete-video="$emit('delete-video')"
      @toggle-private="$emit('toggle-private')"
      @open-profile="$emit('open-profile')"
    />

    <div class="info-overlay">
      <div class="author-line">
        <button class="author-tag" type="button" title="进入主页" @click.stop="$emit('open-profile')">
          @{{ video.author.username || video.author.nickname }}
        </button>
      </div>

      <h3 v-if="video.title" class="video-heading">{{ video.title }}</h3>
      <p v-if="video.description" class="video-caption">{{ video.description }}</p>

      <div class="music-ticker">
        <span class="music-note-icon">♫</span>
        <div class="marquee-track">
          <div class="marquee-content">
            <span>原声 - {{ video.author.nickname || video.author.username }} · 原创音乐</span>
            <span class="marquee-spacer"></span>
            <span>原声 - {{ video.author.nickname || video.author.username }} · 原创音乐</span>
          </div>
        </div>
      </div>
    </div>

    <div class="progress-controller" :class="{ seeking }" @click.stop>
      <div class="scrub-track">
        <div class="scrub-buffered" :style="{ width: `${progressValue}%` }"></div>
        <div class="scrub-filled" :style="{ width: `${progressValue}%` }">
          <span class="scrub-thumb"></span>
        </div>
      </div>
      <input
        class="scrub-input"
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
      <div class="time-bubble">{{ formattedCurrentTime }} / {{ formattedDuration }}</div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import ActionSidebar from "@/components/feed/ActionSidebar.vue";
import { useSoundSetting } from "@/composables/useSoundSetting";
import type { Video } from "@/types/domain";

const props = withDefaults(
  defineProps<{
    video: Video;
    active: boolean;
    showFollow?: boolean;
    showDelete?: boolean;
    showPrivate?: boolean;
    framed?: boolean;
    inWindow?: boolean;
  }>(),
  {
    showFollow: true,
    showDelete: false,
    showPrivate: false,
    framed: false,
    inWindow: true
  }
);

defineEmits<{
  (e: "toggle-like"): void;
  (e: "toggle-favorite"): void;
  (e: "toggle-follow"): void;
  (e: "open-comment"): void;
  (e: "share"): void;
  (e: "delete-video"): void;
  (e: "toggle-private"): void;
  (e: "open-profile"): void;
}>();

const { globalMuted, toggleMuted, setMuted } = useSoundSetting();
const isMuted = globalMuted;

const videoRef = ref<HTMLVideoElement | null>(null);
const isPaused = ref(false);
const currentTime = ref(0);
const duration = ref(0);
const seeking = ref(false);

const effectiveMuted = computed(() => !props.active || globalMuted.value);
const progressValue = computed(() => {
  if (!duration.value) return 0;
  return Math.min(100, Math.max(0, (currentTime.value / duration.value) * 100));
});
const formattedCurrentTime = computed(() => formatTime(currentTime.value));
const formattedDuration = computed(() => formatTime(duration.value));

watch(
  () => [props.active, props.inWindow],
  async ([active, inWindow]) => {
    if (!inWindow) {
      isPaused.value = true;
      return;
    }
    await nextTick();
    if (!videoRef.value) return;
    if (active) {
      videoRef.value.muted = globalMuted.value;
      videoRef.value.volume = globalMuted.value ? 0 : 1;
      try {
        await videoRef.value.play();
        isPaused.value = false;
      } catch {
        setMuted(true);
        videoRef.value.muted = true;
        videoRef.value.volume = 0;
        void videoRef.value.play();
        isPaused.value = false;
      }
      return;
    }
    videoRef.value.pause();
    isPaused.value = true;
    videoRef.value.muted = true;
    videoRef.value.volume = 0;
  },
  { immediate: true }
);

watch(
  () => globalMuted.value,
  (muted) => {
    if (!videoRef.value || !props.active || !props.inWindow) return;
    videoRef.value.muted = muted;
    videoRef.value.volume = muted ? 0 : 1;
  }
);

function onVideoTap() {
  if (!props.active || !props.inWindow) return;
  if (globalMuted.value) {
    setMuted(false);
    if (videoRef.value) {
      videoRef.value.muted = false;
      videoRef.value.volume = 1;
      void videoRef.value.play();
      isPaused.value = false;
    }
    return;
  }
  if (!videoRef.value) return;
  if (videoRef.value.paused) {
    void videoRef.value.play();
    isPaused.value = false;
  } else {
    videoRef.value.pause();
    isPaused.value = true;
  }
}

function toggleMute() {
  if (!props.active) return;
  toggleMuted();
  if (videoRef.value) {
    videoRef.value.muted = globalMuted.value;
    videoRef.value.volume = globalMuted.value ? 0 : 1;
    if (!globalMuted.value) {
      void videoRef.value.play();
      isPaused.value = false;
    }
  }
}

function onLoadedMetadata() {
  if (!videoRef.value) return;
  videoRef.value.muted = effectiveMuted.value;
  videoRef.value.volume = effectiveMuted.value ? 0 : 1;
  duration.value = Number.isFinite(videoRef.value.duration) ? videoRef.value.duration : 0;
  currentTime.value = Number.isFinite(videoRef.value.currentTime) ? videoRef.value.currentTime : 0;
  isPaused.value = videoRef.value.paused;
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
  background-color: #000000;
  display: flex;
  justify-content: center;
  align-items: center;
}

.video-card.framed {
  width: calc(100% - 32px);
  max-width: 1080px;
  height: calc(100svh - 16px);
  margin: 8px auto;
  border-radius: 16px;
  box-shadow: 0 16px 50px rgba(0, 0, 0, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.ambient-backdrop {
  position: absolute;
  inset: -20px;
  overflow: hidden;
  z-index: 1;
  pointer-events: none;
  opacity: 0.25;
  filter: blur(40px);
}

.ambient-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.player-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
}

.video-player {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background-color: transparent;
  border-radius: inherit;
}

.play-state-overlay {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background-color: rgba(0, 0, 0, 0.2);
  cursor: pointer;
  z-index: 5;
}

.big-play-icon {
  font-size: 56px;
  color: rgba(255, 255, 255, 0.7);
  filter: drop-shadow(0 4px 12px rgba(0, 0, 0, 0.6));
  transform: scale(1);
  transition: transform 0.2s ease;
}

.vignette-bottom {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 32%;
  z-index: 4;
  pointer-events: none;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.75) 0%, rgba(0, 0, 0, 0.25) 40%, transparent 100%);
}

.sound-pill {
  position: absolute;
  top: 18px;
  right: 16px;
  z-index: 10;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 999px;
  padding: 6px 12px;
  background-color: rgba(0, 0, 0, 0.45);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  backdrop-filter: blur(8px);
  transition: background-color 0.2s ease;
}

.sound-pill:hover {
  background-color: rgba(0, 0, 0, 0.7);
}

.sound-icon {
  font-size: 14px;
}

.info-overlay {
  position: absolute;
  left: 16px;
  right: 84px;
  bottom: calc(24px + var(--nav-space, 0px));
  z-index: 9;
  display: flex;
  flex-direction: column;
  gap: 6px;
  user-select: text;
}

.author-line {
  display: flex;
  align-items: center;
  gap: 8px;
}

.author-tag {
  padding: 0;
  border: none;
  background-color: transparent;
  color: #ffffff;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.2px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.75);
  cursor: pointer;
}

.video-heading {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #ffffff;
  line-height: 1.35;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
}

.video-caption {
  margin: 0;
  font-size: 14px;
  font-weight: 400;
  color: rgba(255, 255, 255, 0.9);
  line-height: 1.4;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
}

.music-ticker {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  color: rgba(255, 255, 255, 0.88);
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
}

.music-note-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.marquee-track {
  overflow: hidden;
  white-space: nowrap;
  mask-image: linear-gradient(90deg, transparent, #000000 8%, #000000 92%, transparent);
}

.marquee-content {
  display: inline-flex;
  align-items: center;
  animation: marquee-roll 14s linear infinite;
}

.marquee-spacer {
  display: inline-block;
  width: 48px;
}

@keyframes marquee-roll {
  0% {
    transform: translateX(0);
  }
  100% {
    transform: translateX(-50%);
  }
}

.progress-controller {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 14px;
  z-index: 12;
  display: flex;
  align-items: flex-end;
  cursor: pointer;
}

.scrub-track {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 3px;
  background-color: rgba(255, 255, 255, 0.25);
  transition: height 0.15s ease;
}

.progress-controller:hover .scrub-track,
.progress-controller.seeking .scrub-track {
  height: 6px;
}

.scrub-filled {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  background-color: #ffffff;
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

.scrub-thumb {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background-color: #ffffff;
  margin-right: -5px;
  opacity: 0;
  transform: scale(0.6);
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.progress-controller:hover .scrub-thumb,
.progress-controller.seeking .scrub-thumb {
  opacity: 1;
  transform: scale(1);
}

.scrub-input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
  margin: 0;
  z-index: 3;
}

.time-bubble {
  position: absolute;
  bottom: 18px;
  left: 50%;
  transform: translateX(-50%);
  padding: 4px 10px;
  border-radius: 6px;
  background-color: rgba(0, 0, 0, 0.75);
  color: #ffffff;
  font-size: 11px;
  font-weight: 600;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.progress-controller:hover .time-bubble,
.progress-controller.seeking .time-bubble {
  opacity: 1;
}

@media (max-width: 900px) {
  .video-card.framed {
    width: 100%;
    max-width: 100%;
    height: 100svh;
    margin: 0;
    border-radius: 0;
    border: none;
  }

  .sound-pill {
    top: 64px;
    right: 12px;
  }
}
</style>
