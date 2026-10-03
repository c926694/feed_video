<template>
  <div ref="pageRef" class="message-page" @scroll.passive="onScroll">
    <header class="message-header">
      <button class="back-btn" type="button" @click="router.back()">返回</button>
      <h1 class="page-title">消息</h1>
      <button v-if="unreadCount > 0" class="read-all" type="button" @click="readAll">全部已读</button>
      <span v-else class="read-all-placeholder" aria-hidden="true"></span>
    </header>

    <ul v-if="messages.length" class="message-list">
      <MessageItem
        v-for="item in messages"
        :key="item.id"
        :message="item"
        @open="openMessage"
        @open-actor="openActor"
      />
    </ul>

    <p v-else-if="!loading" class="empty-text">还没有收到消息</p>
    <p v-if="loading" class="loading-text">加载中...</p>
    <p v-else-if="noMore && messages.length" class="loading-text">没有更多了</p>

    <BottomNav />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import BottomNav from "@/components/layout/BottomNav.vue";
import MessageItem from "@/components/message/MessageItem.vue";
import { fetchMessageList, markMessagesRead } from "@/api";
import { useToast } from "@/composables/useToast";
import { useUnreadMessages } from "@/composables/useUnreadMessages";
import type { MessageItem as MessageItemType } from "@/types/domain";

const pageSize = 20;

const router = useRouter();
const { showToast } = useToast();
const { unreadCount, refresh: refreshUnread } = useUnreadMessages();

const messages = ref<MessageItemType[]>([]);
const pageRef = ref<HTMLElement | null>(null);
const loading = ref(false);
const noMore = ref(false);
const cursor = ref<{ lastCreatedAt: number; lastId: number }>({ lastCreatedAt: 0, lastId: 0 });

async function loadFirstPage() {
  loading.value = true;
  try {
    const page = await fetchMessageList({ limit: pageSize });
    messages.value = page.messages;
    cursor.value = { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId };
    noMore.value = page.messages.length < pageSize;
  } catch {
    showToast("加载消息失败");
  } finally {
    loading.value = false;
  }
}

async function loadMore() {
  if (loading.value || noMore.value || !cursor.value.lastId) return;
  loading.value = true;
  try {
    const page = await fetchMessageList({ limit: pageSize, ...cursor.value });
    messages.value = [...messages.value, ...page.messages];
    cursor.value = { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId };
    noMore.value = page.messages.length < pageSize;
  } catch {
    showToast("加载更多失败");
  } finally {
    loading.value = false;
  }
}

async function readAll() {
  try {
    await markMessagesRead([]);
    messages.value = messages.value.map((item) => ({ ...item, isRead: true }));
    await refreshUnread();
  } catch {
    showToast("标记失败");
  }
}

// openActor 点通知里的头像进触发者主页
function openActor(actorId: number) {
  if (!actorId) return;
  router.push(`/profile/${actorId}`);
}

// 点击一条：先标记这条已读，再按类型跳转。
// 内容类通知跳单条视频页，评论类把评论 ID 带上，由评论抽屉定位到那一条；
// 关注类跳到对方的资料页
async function openMessage(item: MessageItemType) {
  if (!item.isRead) {
    try {
      await markMessagesRead([item.id]);
      item.isRead = true;
      await refreshUnread();
    } catch {
      showToast("标记失败");
    }
  }
  if (item.type === "follow" && item.actors.length) {
    router.push(`/profile/${item.actors[0].id}`);
    return;
  }
  if (item.videoId > 0) {
    router.push({
      path: `/video/${item.videoId}`,
      ...(item.commentId > 0 ? { query: { comment: item.commentId } } : {})
    });
  }
}

function onScroll() {
  const node = pageRef.value;
  if (!node) return;
  if (node.scrollHeight - (node.scrollTop + node.clientHeight) < 240) {
    void loadMore();
  }
}

onMounted(async () => {
  await loadFirstPage();
  await refreshUnread();
});

onBeforeUnmount(() => {
  // 滚动监听绑在页面容器上，由模板里的 @scroll 负责，这里不需要额外处理
});
</script>

<style scoped>
.message-page {
  height: 100%;
  overflow-y: auto;
  background-color: #000000;
  color: #ffffff;
  padding-bottom: calc(72px + var(--safe-bottom));
}

.message-header {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  background-color: rgba(0, 0, 0, 0.92);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
}

.back-btn {
  border: none;
  background-color: transparent;
  color: rgba(255, 255, 255, 0.72);
  font-size: 14px;
  cursor: pointer;
  padding: 6px 0;
}

.read-all-placeholder {
  width: 72px;
}

.read-all {
  border: 1px solid rgba(255, 255, 255, 0.24);
  background-color: transparent;
  color: #ffffff;
  font-size: 13px;
  padding: 6px 12px;
  border-radius: 999px;
  cursor: pointer;
}

.message-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.empty-text,
.loading-text {
  text-align: center;
  color: rgba(255, 255, 255, 0.45);
  font-size: 13px;
  padding: 24px 16px;
  margin: 0;
}
</style>
