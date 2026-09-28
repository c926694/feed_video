<template>
  <transition name="drawer">
    <section v-if="open" class="mask" @click.self="$emit('close')">
      <div class="sheet">
        <header>
          <h4>评论</h4>
          <button @click="$emit('close')">关闭</button>
        </header>

        <ul class="list">
          <li v-for="item in comments" :key="item.id">
            <div class="author-row">
              <img v-if="item.author.avatar" :src="item.author.avatar" alt="avatar" />
              <span>{{ item.author.username || item.author.nickname }}</span>
              <em v-if="isMine(item)" class="me-badge">我</em>
            </div>
            <p>{{ item.content }}</p>
            <small>
              <button class="like-btn" :class="{ active: item.liked }" @click="onLike(item.id)">
                <svg viewBox="0 0 24 24" class="icon-svg" aria-hidden="true">
                  <path d="M12 20.2 4.9 13.7a4.9 4.9 0 0 1 6.9-7L12 7.9l.2-.2a4.9 4.9 0 0 1 6.9 7L12 20.2Z" />
                </svg>
                <span>{{ item.likeCount }}</span>
              </button>
              <button class="reply-btn" @click="startReply(item.id, 0, item.author.username || item.author.nickname)">回复</button>
              <button v-if="isMine(item)" class="delete-btn" @click="removeComment(item)">删除</button>
            </small>

            <template v-if="replyState(item.id)?.open">
              <ul class="replies">
                <li v-for="reply in replyState(item.id)?.replies" :key="reply.id">
                  <span class="reply-author">
                    <img v-if="reply.author.avatar" :src="reply.author.avatar" alt="avatar" />
                    {{ reply.author.username || reply.author.nickname }}
                  </span>
                  <em v-if="isMine(reply)" class="me-badge">我</em>
                  <span v-if="reply.replyToUserName" class="reply-to">回复 @{{ reply.replyToUserName }}</span>
                  <p>{{ reply.content }}</p>
                  <small>
                    <button class="like-btn" :class="{ active: reply.liked }" @click="onReplyLike(item.id, reply.id)">
                      <svg viewBox="0 0 24 24" class="icon-svg" aria-hidden="true">
                        <path d="M12 20.2 4.9 13.7a4.9 4.9 0 0 1 6.9-7L12 7.9l.2-.2a4.9 4.9 0 0 1 6.9 7L12 20.2Z" />
                      </svg>
                      <span>{{ reply.likeCount }}</span>
                    </button>
                    <button class="reply-btn" @click="startReply(item.id, reply.id, reply.author.username || reply.author.nickname)">回复</button>
                    <button v-if="isMine(reply)" class="delete-btn" @click="removeComment(reply)">删除</button>
                  </small>
                </li>
              </ul>
              <button v-if="replyState(item.id)?.hasMore" class="more-btn" @click="loadMoreReplies(item)">
                展开回复
              </button>
              <button class="more-btn" @click="collapseReplies(item.id)">收起</button>
            </template>
            <button
              v-else-if="item.replyCount > 0"
              class="more-btn"
              @click="expandReplies(item)"
            >
              展开 {{ item.replyCount }} 条回复
            </button>
          </li>
        </ul>
        <button v-if="hasMore" class="load-btn" @click="loadMore">加载更多评论</button>

        <form class="composer" @submit.prevent="submitComment">
          <input
            v-model.trim="draft"
            :placeholder="replyTarget ? `回复 @${replyTarget.name}` : '写下你的评论...'"
          />
          <button v-if="replyTarget" class="cancel-reply" type="button" @click="replyTarget = null">取消</button>
          <button :disabled="sending || !draft" type="submit">{{ sending ? "发送中" : "发送" }}</button>
        </form>
      </div>
    </section>
  </transition>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { createComment, deleteComment, fetchCommentList, fetchMe, fetchReplyList, setCommentLike } from "@/api";
import type { Comment } from "@/types/domain";
import { useToast } from "@/composables/useToast";

const props = defineProps<{
  open: boolean;
  videoId: number;
}>();

defineEmits<{
  (e: "close"): void;
}>();

const { showToast } = useToast();

const comments = ref<Comment[]>([]);
const draft = ref("");
const sending = ref(false);
const cursor = ref<{ lastCreatedAt: number; lastId: number } | null>(null);
const hasMore = ref(false);
const replyTarget = ref<{ id: number; replyToId: number; name: string } | null>(null);
const currentUserId = ref<number | null>(null);

// 每个顶级评论的展开状态：首次展开 3 条，之后每次追加 10 条
interface ReplyState {
  open: boolean;
  replies: Comment[];
  cursor: { lastCreatedAt: number; lastId: number } | null;
  hasMore: boolean;
}
const replyStates = ref<Record<number, ReplyState>>({});
const firstExpandSize = 3;
const moreExpandSize = 10;

function replyState(commentId: number): ReplyState | undefined {
  return replyStates.value[commentId];
}

async function loadFirstPage() {
  const page = await fetchCommentList(props.videoId);
  comments.value = page.comments;
  cursor.value = page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null;
  hasMore.value = page.hasMore;
  replyStates.value = {};
}

watch(
  () => props.open,
  async (isOpen) => {
    if (!isOpen || !props.videoId) return;
    replyTarget.value = null;
    try {
      const me = await fetchMe();
      currentUserId.value = me.id;
    } catch {
      currentUserId.value = null;
    }
    await loadFirstPage();
  },
  { immediate: true }
);

async function loadMore() {
  if (!cursor.value) return;
  const page = await fetchCommentList(props.videoId, cursor.value);
  comments.value = comments.value.concat(page.comments);
  cursor.value = page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null;
  hasMore.value = page.hasMore;
}

async function expandReplies(item: Comment) {
  const existing = replyState(item.id);
  if (existing) {
    replyStates.value = { ...replyStates.value, [item.id]: { ...existing, open: true } };
    return;
  }
  const page = await fetchReplyList(item.id, { limit: firstExpandSize });
  replyStates.value = {
    ...replyStates.value,
    [item.id]: {
      open: true,
      replies: page.comments,
      cursor: page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null,
      hasMore: page.hasMore
    }
  };
}

async function loadMoreReplies(item: Comment) {
  const state = replyState(item.id);
  if (!state?.cursor) return;
  const page = await fetchReplyList(item.id, { limit: moreExpandSize, cursor: state.cursor });
  replyStates.value = {
    ...replyStates.value,
    [item.id]: {
      ...state,
      replies: state.replies.concat(page.comments),
      cursor: page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null,
      hasMore: page.hasMore
    }
  };
}

function collapseReplies(commentId: number) {
  const state = replyState(commentId);
  if (!state) return;
  replyStates.value = { ...replyStates.value, [commentId]: { ...state, open: false } };
}

function startReply(commentId: number, replyToId: number, name: string) {
  replyTarget.value = { id: commentId, replyToId, name };
}

async function submitComment() {
  if (!draft.value || !props.videoId) return;
  sending.value = true;
  try {
    await createComment({
      video_id: props.videoId,
      content: draft.value,
      ...(replyTarget.value ? { parent_id: replyTarget.value.id, reply_to_id: replyTarget.value.replyToId } : {})
    });
    draft.value = "";
    replyTarget.value = null;
    await loadFirstPage();
    showToast("评论成功");
  } finally {
    sending.value = false;
  }
}

function isMine(comment: Comment) {
  return currentUserId.value !== null && comment.author.id === currentUserId.value;
}

async function removeComment(item: Comment) {
  const isTop = item.parentId === 0;
  const message =
    isTop && item.replyCount > 0
      ? `删除这条评论及其 ${item.replyCount} 条回复？`
      : "删除这条评论？";
  if (!window.confirm(message)) return;
  try {
    await deleteComment(item.id);
  } catch {
    showToast("删除失败，请重试");
    return;
  }

  if (isTop) {
    comments.value = comments.value.filter((entry) => entry.id !== item.id);
    const next = { ...replyStates.value };
    delete next[item.id];
    replyStates.value = next;
  } else {
    const parentId = item.parentId;
    comments.value = comments.value.map((entry) =>
      entry.id === parentId
        ? { ...entry, replyCount: Math.max(0, entry.replyCount - 1) }
        : entry
    );
    const state = replyState(parentId);
    if (state) {
      replyStates.value = {
        ...replyStates.value,
        [parentId]: {
          ...state,
          replies: state.replies.filter((entry) => entry.id !== item.id)
        }
      };
    }
  }
  showToast("删除成功");
}

async function onLike(commentId: number) {
  const current = comments.value.find((item) => item.id === commentId);
  if (!current) return;
  const targetLiked = await setCommentLike(commentId, !current.liked);
  comments.value = comments.value.map((item) =>
    item.id === commentId
      ? {
          ...item,
          liked: targetLiked,
          likeCount: Math.max(0, item.likeCount + ((targetLiked ? 1 : 0) - (item.liked ? 1 : 0)))
        }
      : item
  );
}

async function onReplyLike(commentId: number, replyId: number) {
  const state = replyState(commentId);
  const current = state?.replies.find((item) => item.id === replyId);
  if (!state || !current) return;
  const targetLiked = await setCommentLike(replyId, !current.liked);
  replyStates.value = {
    ...replyStates.value,
    [commentId]: {
      ...state,
      replies: state.replies.map((item) =>
        item.id === replyId
          ? {
              ...item,
              liked: targetLiked,
              likeCount: Math.max(0, item.likeCount + ((targetLiked ? 1 : 0) - (item.liked ? 1 : 0)))
            }
          : item
      )
    }
  };
}
</script>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.42);
  z-index: 90;
}

.sheet {
  position: absolute;
  top: 0;
  right: 0;
  width: min(420px, 34vw);
  height: 100svh;
  background: rgba(15, 19, 34, 0.96);
  border-left: 1px solid var(--line);
  box-shadow: -24px 0 60px rgba(0, 0, 0, 0.32);
  display: flex;
  flex-direction: column;
}

header {
  padding: 14px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--line);
}

h4 {
  margin: 0;
}

header button {
  border: none;
  background: transparent;
  color: var(--text-muted);
}

.list {
  flex: 1;
  margin: 0;
  padding: 0 14px;
  overflow: auto;
  list-style: none;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.list::-webkit-scrollbar {
  display: none;
}

li {
  padding: 12px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.author-row {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 12px;
}

.author-row img {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  object-fit: cover;
}

p {
  margin: 8px 0;
}

.like-btn,
.reply-btn {
  border: none;
  color: var(--text-muted);
  background: transparent;
  padding: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.like-btn.active {
  color: #ff5d7a;
}

.reply-btn {
  margin-left: 14px;
}

.delete-btn {
  border: none;
  background: transparent;
  color: var(--text-muted);
  padding: 0;
  margin-left: 14px;
}

.me-badge {
  background: var(--accent);
  color: #fff;
  font-style: normal;
  font-size: 10px;
  line-height: 1;
  padding: 2px 5px;
  border-radius: 999px;
}

.icon-svg {
  width: 16px;
  height: 16px;
  fill: currentColor;
}

.replies {
  margin: 8px 0 0;
  padding: 8px 0 0 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.04);
  list-style: none;
}

.replies li {
  padding: 8px 0;
  border-bottom: none;
}

.reply-author {
  color: var(--text-muted);
  font-size: 12px;
}

.reply-author img {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  object-fit: cover;
  vertical-align: middle;
  margin-right: 6px;
}

.reply-to {
  color: var(--text-muted);
  font-size: 12px;
}

.replies p {
  margin: 4px 0;
}

.more-btn,
.load-btn {
  border: none;
  background: transparent;
  color: var(--accent);
  font-size: 12px;
  padding: 6px 0;
  display: block;
}

.load-btn {
  margin: 0 14px;
}

.composer {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 8px;
  padding: 12px 14px calc(12px + var(--safe-bottom));
  border-top: 1px solid var(--line);
}

input {
  border: 1px solid var(--line);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.04);
  color: #fff;
  padding: 10px 12px;
}

.composer button {
  border: none;
  border-radius: 999px;
  background: var(--accent);
  color: #fff;
  padding: 0 16px;
}

.composer .cancel-reply {
  background: transparent;
  color: var(--text-muted);
  border: 1px solid var(--line);
}

.drawer-enter-active,
.drawer-leave-active {
  transition: opacity 0.2s ease;
}

.drawer-enter-from,
.drawer-leave-to {
  opacity: 0;
}

.drawer-enter-active .sheet,
.drawer-leave-active .sheet {
  transition: transform 0.24s ease;
}

.drawer-enter-from .sheet,
.drawer-leave-to .sheet {
  transform: translateX(100%);
}

@media (max-width: 900px) {
  .sheet {
    width: min(100vw, 380px);
  }
}

@media (max-width: 640px) {
  .sheet {
    width: 100vw;
  }
}
</style>
