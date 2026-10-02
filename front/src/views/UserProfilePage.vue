<template>
  <section class="user-profile-page">
    <header class="top-bar">
      <button class="back-btn" type="button" @click="goBack">←</button>
      <h1>主页</h1>
    </header>

    <ProfileHeader v-if="user" :user="user" :video-count="videos.length" />
    <p v-else-if="loading" class="loading">正在加载用户信息...</p>
    <p v-else class="loading">用户不存在</p>

    <div class="panel">
      <p v-if="listLoading" class="loading">加载中...</p>
      <UserVideoGrid v-else :videos="videos" empty-text="TA 还没有发布视频" :manage="false" source="works" />
    </div>

    <BottomNav />
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { fetchAuthorVideos, fetchUserProfile } from "@/api";
import BottomNav from "@/components/layout/BottomNav.vue";
import ProfileHeader from "@/components/profile/ProfileHeader.vue";
import UserVideoGrid from "@/components/profile/UserVideoGrid.vue";
import { useVideoActions } from "@/composables/useVideoActions";
import type { User, Video } from "@/types/domain";

const route = useRoute();
const router = useRouter();

const user = ref<User | null>(null);
const videos = ref<Video[]>([]);
const loading = ref(true);
const listLoading = ref(true);

useVideoActions(() => [videos.value]);

async function loadPage() {
  const userId = Number(route.params.id ?? 0);
  if (!userId) {
    loading.value = false;
    listLoading.value = false;
    return;
  }
  loading.value = true;
  listLoading.value = true;
  try {
    user.value = await fetchUserProfile(userId);
  } catch {
    user.value = null;
  } finally {
    loading.value = false;
  }
  try {
    const page = await fetchAuthorVideos(userId, { limit: 60 });
    videos.value = page.videos;
  } catch {
    videos.value = [];
  } finally {
    listLoading.value = false;
  }
}

function goBack() {
  if (window.history.length > 1) {
    router.back();
    return;
  }
  router.push("/feed");
}

onMounted(loadPage);
watch(() => route.params.id, loadPage);
</script>

<style scoped>
.user-profile-page {
  min-height: 100vh;
  background-color: #000000;
  color: #ffffff;
  padding-bottom: calc(72px + var(--safe-bottom));
}

.top-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
}

.top-bar h1 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
}

.back-btn {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: none;
  background-color: rgba(255, 255, 255, 0.08);
  color: #ffffff;
  font-size: 16px;
  cursor: pointer;
}

.panel {
  padding: 0 16px;
}

.loading {
  color: rgba(255, 255, 255, 0.55);
  font-size: 13px;
  padding: 20px 16px;
}
</style>
