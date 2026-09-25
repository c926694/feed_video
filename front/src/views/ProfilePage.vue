<template>
  <section class="profile-page">
    <header class="top-bar">
      <h1>我的</h1>
      <div class="top-actions">
        <button @click="toggleEdit">{{ editing ? "取消编辑" : "编辑资料" }}</button>
        <button @click="onLogout">退出登录</button>
      </div>
    </header>

    <ProfileHeader v-if="currentUser" :user="currentUser" :video-count="myVideos.length" />
    <p v-else class="loading">正在加载个人信息...</p>

    <form v-if="currentUser && editing" class="edit-panel" @submit.prevent="onSaveProfile">
      <label>
        昵称
        <input v-model.trim="form.nickname" placeholder="输入新的昵称" />
      </label>
      <label>
        头像
        <input type="file" accept="image/*" @change="onAvatarChange" />
      </label>
      <img v-if="avatarPreview" :src="avatarPreview" alt="avatar-preview" class="avatar-preview" />
      <button :disabled="saving" type="submit">{{ saving ? "保存中..." : "保存资料" }}</button>
    </form>

    <UserVideoGrid :videos="myVideos" />

    <BottomNav />
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import { fetchMe, fetchMyVideos, logout, updateMyProfile } from "@/api";
import BottomNav from "@/components/layout/BottomNav.vue";
import ProfileHeader from "@/components/profile/ProfileHeader.vue";
import UserVideoGrid from "@/components/profile/UserVideoGrid.vue";
import { useAuth } from "@/composables/useAuth";
import type { User, Video } from "@/types/domain";
import { useToast } from "@/composables/useToast";

const router = useRouter();
const { clearAuth } = useAuth();
const { showToast } = useToast();

const currentUser = ref<User | null>(null);
const myVideos = ref<Video[]>([]);
const editing = ref(false);
const saving = ref(false);
const avatarFile = ref<File | null>(null);
const avatarPreview = ref("");
const form = ref({
  nickname: ""
});

async function bootstrap() {
  try {
    currentUser.value = await fetchMe();
    myVideos.value = await fetchMyVideos(120);
    if (currentUser.value) {
      currentUser.value.videoCount = Math.max(currentUser.value.videoCount, myVideos.value.length);
      form.value.nickname = currentUser.value.nickname;
    }
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  }
}

function clearAvatarPreview() {
  if (avatarPreview.value) {
    URL.revokeObjectURL(avatarPreview.value);
  }
  avatarPreview.value = "";
}

function resetEditState() {
  avatarFile.value = null;
  clearAvatarPreview();
  form.value.nickname = currentUser.value?.nickname ?? "";
}

function toggleEdit() {
  editing.value = !editing.value;
  if (!editing.value) {
    resetEditState();
  }
}

function onAvatarChange(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0] ?? null;
  avatarFile.value = file;
  clearAvatarPreview();
  avatarPreview.value = file ? URL.createObjectURL(file) : "";
}

async function onSaveProfile() {
  if (!currentUser.value) return;
  const nickname = form.value.nickname.trim();
  if (!nickname && !avatarFile.value) {
    showToast("请至少修改昵称或头像");
    return;
  }

  saving.value = true;
  try {
    const user = await updateMyProfile({
      nickname,
      avatar: avatarFile.value
    });
    user.videoCount = Math.max(user.videoCount, myVideos.value.length);
    currentUser.value = user;
    editing.value = false;
    resetEditState();
    showToast("资料已更新");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    saving.value = false;
  }
}

async function onLogout() {
  try {
    await logout();
    showToast("已退出登录");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    clearAuth();
    router.push("/login");
  }
}

onMounted(() => {
  void bootstrap();
});

onUnmounted(() => {
  clearAvatarPreview();
});
</script>

<style scoped>
.profile-page {
  min-height: 100svh;
}

.top-bar {
  padding: 16px 14px 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.top-actions {
  display: flex;
  gap: 8px;
}

h1 {
  margin: 0;
}

button {
  border: 1px solid var(--line);
  border-radius: 999px;
  background: transparent;
  color: var(--text-primary);
  padding: 6px 12px;
}

.loading {
  padding: 12px 16px;
  color: var(--text-muted);
}

.edit-panel {
  margin: 10px 14px 0;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.edit-panel label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
}

.edit-panel input {
  border: 1px solid var(--line);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  color: #fff;
  padding: 10px;
}

.avatar-preview {
  width: 84px;
  height: 84px;
  border-radius: 50%;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.25);
}
</style>
