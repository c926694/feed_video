<template>
  <form class="panel" @submit.prevent="submit">
    <h2>发布视频</h2>
    <p>选择封面和视频文件，点击发布后自动开始上传，可在个人主页查看进度。</p>

    <label>
      标题
      <input v-model.trim="form.title" required />
    </label>
    <label>
      描述
      <textarea v-model.trim="form.description" rows="4" placeholder="写一点视频介绍，方便 feed 展示。" />
    </label>
    <label>
      封面文件
      <input type="file" accept="image/*" required @change="onCoverChange" />
    </label>
    <label>
      视频文件
      <input type="file" accept="video/*" required @change="onPlayChange" />
    </label>

    <button :disabled="loading" type="submit">{{ loading ? "发布中..." : "发布" }}</button>
  </form>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { COVER_MAX_SIZE, VIDEO_MAX_SIZE, startUpload } from "@/composables/useUploadQueue";
import { useToast } from "@/composables/useToast";

const router = useRouter();
const { showToast } = useToast();
const loading = ref(false);
const form = reactive({
  title: "",
  description: "",
  cover: null as File | null,
  play: null as File | null
});

async function submit() {
  if (!form.cover || !form.play) return;
  loading.value = true;
  try {
    await startUpload({
      title: form.title,
      description: form.description,
      coverFile: form.cover,
      playFile: form.play
    });
    showToast("已开始上传，完成后自动发布");
    router.push("/profile");
  } finally {
    loading.value = false;
  }
}

function onCoverChange(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0] ?? null;
  if (file && file.size > COVER_MAX_SIZE) {
    showToast("封面不能超过 10MB");
    target.value = "";
    form.cover = null;
    return;
  }
  form.cover = file;
}

function onPlayChange(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0] ?? null;
  if (file && file.size > VIDEO_MAX_SIZE) {
    showToast("视频不能超过 10GB");
    target.value = "";
    form.play = null;
    return;
  }
  form.play = file;
}
</script>

<style scoped>
.panel {
  width: min(560px, 94vw);
  margin: 24px auto 120px;
  padding: 18px;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: rgba(8, 12, 20, 0.78);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

h2 {
  margin: 0;
}

p {
  margin: 0;
  color: var(--text-muted);
  font-size: 13px;
}

label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
}

input {
  border: 1px solid var(--line);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  padding: 10px;
  color: #fff;
}

textarea {
  border: 1px solid var(--line);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  padding: 10px;
  color: #fff;
  resize: vertical;
  min-height: 92px;
}

button {
  border: none;
  border-radius: 999px;
  padding: 10px;
  color: #fff;
  background: linear-gradient(90deg, #18b6ff, #42d77d);
}

button:disabled {
  opacity: 0.6;
}
</style>
