<template>
  <section ref="pageRef" class="user-profile-page">
    <header class="top-bar">
      <button class="back-btn" type="button" @click="goBack">←</button>
      <h1>主页</h1>
    </header>

    <ProfileHeader
      v-if="user"
      :user="user"
      :video-count="videos.length"
      @open-following="router.push(`/profile/${profileUserId}/following`)"
      @open-followers="router.push(`/profile/${profileUserId}/followers`)"
    />
    <p v-else-if="loading" class="loading">正在加载用户信息...</p>
    <p v-else class="loading">用户不存在</p>

    <div v-if="user" class="follow-bar">
      <button class="follow-btn" :class="{ followed: user.followed }" :disabled="followPending" @click="toggleFollow">
        {{ user.followed ? "已关注" : "关注" }}
      </button>
    </div>

    <div class="panel">
      <p v-if="listLoading" class="loading">加载中...</p>
      <template v-else>
        <UserVideoGrid :videos="videos" empty-text="TA 还没有发布视频" :manage="false" source="works" :owner-id="profileUserId" />
        <p v-if="moreLoading" class="loading">加载中...</p>
        <p v-else-if="!hasMore && videos.length" class="loading">没有更多了</p>
      </template>
    </div>

    <BottomNav />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { fetchAuthorVideos, fetchMe, fetchUserProfile, setFollow } from "@/api";
import BottomNav from "@/components/layout/BottomNav.vue";
import ProfileHeader from "@/components/profile/ProfileHeader.vue";
import UserVideoGrid from "@/components/profile/UserVideoGrid.vue";
import { useLoadMoreOnScroll } from "@/composables/useLoadMoreOnScroll";
import { useToast } from "@/composables/useToast";
import { useVideoActions } from "@/composables/useVideoActions";
import type { User, Video } from "@/types/domain";

const route = useRoute();
const router = useRouter();
const { showToast } = useToast();

const user = ref<User | null>(null);
const pageRef = ref<HTMLElement | null>(null);
const videos = ref<Video[]>([]);
const loading = ref(true);
const listLoading = ref(true);
const followPending = ref(false);
const profileUserId = computed(() => Number(route.params.id ?? 0));
const authorPageSize = 20;
const authorCursor = ref<{ lastCreatedAt: number; lastId: number } | null>(null);
const hasMore = ref(false);

useVideoActions(() => [videos.value]);

// toggleFollow 关注或取关，成功后本地更新按钮状态与他的粉丝数
async function toggleFollow() {
  const current = user.value;
  if (!current || followPending.value) return;
  followPending.value = true;
  try {
    const followed = await setFollow(current.id, !current.followed);
    user.value = {
      ...current,
      followed,
      followerCount: Math.max(0, current.followerCount + (followed ? 1 : 0) - (current.followed ? 1 : 0))
    };
    showToast(followed ? "已关注" : "已取消关注");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    followPending.value = false;
  }
}

async function loadPage() {
  const userId = Number(route.params.id ?? 0);
  if (!userId) {
    loading.value = false;
    listLoading.value = false;
    return;
  }

  // 点自己的头像或昵称时回到自己的主页，那里有编辑资料等入口
  try {
    const me = await fetchMe();
    if (me.id === userId) {
      router.replace("/profile");
      return;
    }
  } catch {
    // 拿不到登录用户就按别人的主页处理
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
    const page = await fetchAuthorVideos(userId, { limit: authorPageSize });
    videos.value = page.videos;
    authorCursor.value = page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null;
    hasMore.value = page.hasMore;
  } catch {
    videos.value = [];
    hasMore.value = false;
  } finally {
    listLoading.value = false;
  }
}

// loadMoreVideos 滚到底时接上作者的下一页
async function loadMoreVideos() {
  const userId = profileUserId.value;
  if (!userId) return;
  const page = await fetchAuthorVideos(userId, {
    limit: authorPageSize,
    lastCreatedAt: authorCursor.value?.lastCreatedAt,
    lastId: authorCursor.value?.lastId
  });
  videos.value = [...videos.value, ...page.videos];
  authorCursor.value = page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null;
  hasMore.value = page.hasMore;
}

const { loading: moreLoading } = useLoadMoreOnScroll(
  () => pageRef.value,
  () => hasMore.value && !listLoading.value,
  loadMoreVideos
);

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
  height: 100%;
  overflow-y: auto;
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

/* 别人主页上的关注与取关 */
.follow-bar {
  display: flex;
  justify-content: center;
  padding: 4px 16px 16px;
}

.follow-btn {
  min-width: 120px;
  padding: 9px 20px;
  border-radius: 999px;
  border: 1px solid var(--tiktok-red);
  background-color: var(--tiktok-red);
  color: #ffffff;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}

.follow-btn.followed {
  background-color: transparent;
  color: rgba(255, 255, 255, 0.85);
  border-color: rgba(255, 255, 255, 0.28);
}

.follow-btn:disabled {
  opacity: 0.6;
  cursor: default;
}

.loading {
  color: rgba(255, 255, 255, 0.55);
  font-size: 13px;
  padding: 20px 16px;
}
</style>
