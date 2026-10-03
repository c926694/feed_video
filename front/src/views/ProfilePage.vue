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

    <div class="tabs" role="tablist">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        class="tab"
        :class="{ active: activeTab === tab.key }"
        role="tab"
        :aria-selected="activeTab === tab.key"
        @click="switchTab(tab.key)"
      >
        {{ tab.label }}
      </button>
    </div>

    <div class="panel">
      <p v-if="listLoading" class="loading">加载中...</p>
      <UserVideoGrid
        v-else
        :videos="activeVideos"
        :empty-text="activeTabMeta.emptyText"
        :manage="activeTab === 'works'"
        :source="activeTab"
        @changed="bootstrap"
      />
    </div>

    <BottomNav />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { fetchAllVideoPages, fetchMe, fetchMyFavorites, fetchMyLikes, fetchMyVideos, logout, updateMyProfile } from "@/api";
import BottomNav from "@/components/layout/BottomNav.vue";
import ProfileHeader from "@/components/profile/ProfileHeader.vue";
import UserVideoGrid from "@/components/profile/UserVideoGrid.vue";
import { useAuth } from "@/composables/useAuth";
import type { User, Video } from "@/types/domain";
import { useToast } from "@/composables/useToast";

const tabs = [
  { key: "works", label: "作品", emptyText: "还没有可展示的视频，先去发布一个吧。" },
  { key: "private", label: "私密作品", emptyText: "还没有私密作品，在作品里把视频设为私密即可。" },
  { key: "favorites", label: "收藏", emptyText: "还没有收藏的视频。" },
  { key: "likes", label: "点赞", emptyText: "还没有点赞的视频。" }
] as const;

type TabKey = (typeof tabs)[number]["key"];

const router = useRouter();
const route = useRoute();
const { clearAuth, refreshToken } = useAuth();
const { showToast } = useToast();

const currentUser = ref<User | null>(null);
const myVideos = ref<Video[]>([]);
const favoriteVideos = ref<Video[]>([]);
const likeVideos = ref<Video[]>([]);
const activeTab = ref<TabKey>("works");
const listLoading = ref(false);
const editing = ref(false);
const saving = ref(false);
const avatarFile = ref<File | null>(null);
const avatarPreview = ref("");
const form = ref({
  nickname: ""
});

const activeTabMeta = computed(() => tabs.find((tab) => tab.key === activeTab.value) ?? tabs[0]);
const activeVideos = computed(() => {
  if (activeTab.value === "favorites") return favoriteVideos.value;
  if (activeTab.value === "likes") return likeVideos.value;
  if (activeTab.value === "private") return myVideos.value.filter((video) => video.status === "private");
  return myVideos.value.filter((video) => video.status !== "private");
});

function parseTab(raw: unknown): TabKey {
  if (raw === "favorites" || raw === "likes" || raw === "private") return raw;
  return "works";
}

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

// 每次切到收藏或点赞都重新拉一遍，播放页里取消操作后回来看到的就是最新列表；
// 私密作品那一栏用同一份我的视频，只重新拉一次保证隐藏操作后的状态是最新的
async function switchTab(key: TabKey) {
  activeTab.value = key;
  if (key === "works") return;

  listLoading.value = true;
  try {
    if (key === "private") {
      myVideos.value = await fetchMyVideos(120);
    } else if (key === "favorites") {
      favoriteVideos.value = await fetchAllVideoPages(fetchMyFavorites);
    } else {
      likeVideos.value = await fetchAllVideoPages(fetchMyLikes);
    }
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    listLoading.value = false;
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
    await logout(refreshToken.value);
    showToast("已退出登录");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    clearAuth();
    router.push("/login");
  }
}

onMounted(() => {
  const initialTab = parseTab(route.query.tab);
  if (initialTab !== "works") {
    void switchTab(initialTab);
  }
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

.tabs {
  margin: 14px 14px 0;
  max-width: 760px;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid var(--line);
  border-radius: 14px 14px 0 0;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.03);
}

.tab {
  padding: 12px 0;
  border: none;
  border-right: 1px solid var(--line);
  border-radius: 0;
  background: transparent;
  color: var(--text-muted);
  font-size: 14px;
  cursor: pointer;
}

.tab:last-child {
  border-right: none;
}

.tab.active {
  background: rgba(255, 255, 255, 0.09);
  color: var(--text-primary);
  font-weight: 600;
}

/* 面板与页签共用一条边框，拼成一个分段控件 */
.panel {
  margin: -1px 14px 120px;
  max-width: 760px;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 0 0 14px 14px;
  background: rgba(255, 255, 255, 0.02);
}

@media (min-width: 800px) {
  .tabs,
  .panel {
    margin-left: auto;
    margin-right: auto;
  }
}
</style>
