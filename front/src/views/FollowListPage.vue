<template>
  <section ref="pageRef" class="follow-list-page">
    <header class="top-bar">
      <button class="back-btn" type="button" @click="goBack">←</button>
      <h1>{{ isFollowers ? "粉丝" : "关注" }}</h1>
    </header>

    <ul v-if="users.length" class="user-list">
      <li v-for="item in users" :key="item.id" class="user-item">
        <button class="user-entry" type="button" @click="openProfile(item.id)">
          <img v-if="item.avatar" class="user-avatar" :src="item.avatar" :alt="item.nickname" />
          <span v-else class="user-avatar fallback">{{ item.nickname.slice(0, 1) }}</span>
          <span class="user-name">{{ item.nickname }}</span>
          <em v-if="item.id === currentUserId" class="me-badge">我</em>
        </button>
        <button
          v-if="item.id !== currentUserId"
          class="follow-btn"
          :class="{ followed: item.followed }"
          :disabled="pendingId === item.id"
          @click="toggleFollow(item)"
        >
          {{ item.followed ? "已关注" : "关注" }}
        </button>
      </li>
    </ul>

    <p v-else-if="!loading" class="state-text">{{ isFollowers ? "还没有粉丝" : "还没有关注的人" }}</p>
    <p v-if="loading || moreLoading" class="state-text">加载中...</p>
    <p v-else-if="!hasMore && users.length" class="state-text">没有更多了</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { fetchFollowersList, fetchFollowingList, fetchMe, setFollow } from "@/api";
import { useLoadMoreOnScroll } from "@/composables/useLoadMoreOnScroll";
import { useToast } from "@/composables/useToast";
import type { FollowUser } from "@/types/domain";

const route = useRoute();
const router = useRouter();
const { showToast } = useToast();

const pageRef = ref<HTMLElement | null>(null);
const users = ref<FollowUser[]>([]);
const loading = ref(false);
const hasMore = ref(false);
const cursor = ref<{ lastId: number } | null>(null);
const pendingId = ref(0);
const currentUserId = ref(0);
const pageSize = 20;

// 路由末尾是 followers 就是粉丝列表，否则是关注列表
const isFollowers = computed(() => route.path.endsWith("/followers"));
const profileUserId = computed(() => Number(route.params.id ?? 0));

function loadPage() {
  if (isFollowers.value) {
    return fetchFollowersList(profileUserId.value, { limit: pageSize });
  }
  return fetchFollowingList(profileUserId.value, { limit: pageSize });
}

async function bootstrap() {
  loading.value = true;
  try {
    const page = await loadPage();
    users.value = page.users;
    cursor.value = page.lastId ? { lastId: page.lastId } : null;
    hasMore.value = page.hasMore;
  } catch {
    users.value = [];
    hasMore.value = false;
  } finally {
    loading.value = false;
  }
}

async function loadMore() {
  if (!cursor.value) return;
  const page = isFollowers.value
    ? await fetchFollowersList(profileUserId.value, { limit: pageSize, lastId: cursor.value.lastId })
    : await fetchFollowingList(profileUserId.value, { limit: pageSize, lastId: cursor.value.lastId });
  users.value = [...users.value, ...page.users];
  cursor.value = page.lastId ? { lastId: page.lastId } : null;
  hasMore.value = page.hasMore;
}

const { loading: moreLoading } = useLoadMoreOnScroll(
  () => pageRef.value,
  () => hasMore.value && !loading.value,
  loadMore
);

// toggleFollow 关注或取关列表里的这个人，成功后只改这一行
async function toggleFollow(item: FollowUser) {
  if (pendingId.value) return;
  pendingId.value = item.id;
  try {
    const followed = await setFollow(item.id, !item.followed);
    users.value = users.value.map((row) => (row.id === item.id ? { ...row, followed } : row));
    showToast(followed ? "已关注" : "已取消关注");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    pendingId.value = 0;
  }
}

function openProfile(userId: number) {
  if (!userId) return;
  router.push(`/profile/${userId}`);
}

function goBack() {
  if (profileUserId.value) {
    router.push(`/profile/${profileUserId.value}`);
    return;
  }
  router.push("/feed");
}

onMounted(async () => {
  try {
    const me = await fetchMe();
    currentUserId.value = me.id;
  } catch {
    currentUserId.value = 0;
  }
  await bootstrap();
});

// 在两个列表之间切换时重新取第一页并回到顶部
watch(
  () => route.path,
  async () => {
    cursor.value = null;
    users.value = [];
    pageRef.value?.scrollTo({ top: 0 });
    await bootstrap();
  }
);
</script>

<style scoped>
.follow-list-page {
  height: 100%;
  overflow-y: auto;
  background-color: #000000;
  color: #ffffff;
  padding-bottom: calc(24px + var(--safe-bottom));
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

.user-list {
  list-style: none;
  margin: 0;
  padding: 0 16px;
}

.user-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.user-entry {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 0;
  border: none;
  background-color: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

.user-avatar {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  object-fit: cover;
  background-color: #1f1f1f;
}

.user-avatar.fallback {
  display: grid;
  place-items: center;
  font-size: 16px;
  color: rgba(255, 255, 255, 0.75);
}

.user-name {
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.me-badge {
  padding: 1px 6px;
  border-radius: 4px;
  background-color: rgba(255, 255, 255, 0.12);
  color: rgba(255, 255, 255, 0.8);
  font-size: 11px;
  font-style: normal;
}

.follow-btn {
  min-width: 76px;
  padding: 7px 14px;
  border-radius: 999px;
  border: 1px solid var(--tiktok-red);
  background-color: var(--tiktok-red);
  color: #ffffff;
  font-size: 13px;
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

.state-text {
  padding: 24px 16px;
  margin: 0;
  text-align: center;
  color: rgba(255, 255, 255, 0.5);
  font-size: 13px;
}
</style>
