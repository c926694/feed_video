<template>
  <transition name="drawer">
    <section v-if="open" class="mask" @click.self="$emit('close')">
      <div class="sheet">
        <header>
          <h4>评论 ({{ comments.length }})</h4>
          <button class="close-btn" @click="$emit('close')">✕</button>
        </header>

        <ul class="list">
          <li v-for="item in comments" :key="item.id" :data-comment-id="item.id" :class="{ focused: item.id === highlightId }">
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
                <li
                  v-for="reply in replyState(item.id)?.replies"
                  :key="reply.id"
                  :data-comment-id="reply.id"
                  :class="{ focused: reply.id === highlightId }"
                >
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
import { nextTick, ref, watch } from "vue";
import { createComment, deleteComment, fetchComment, fetchCommentList, fetchMe, fetchReplyList, setCommentLike } from "@/api";
import type { Comment } from "@/types/domain";
import { useToast } from "@/composables/useToast";

const props = defineProps<{
  open: boolean;
  videoId: number;
  // focusCommentId 从通知跳转过来时要定位到的那条评论
  focusCommentId?: number;
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
// 定位到的那条评论，用于滚动与高亮
const highlightId = ref(0);

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

// 定位某条评论。分页游标对 id 是"小于"，所以把目标的 id 加一传给接口，
// 目标自己就会被包含进这一页的第一条
function cursorBefore(comment: Comment) {
  const createdAt = new Date(comment.createdAt).getTime();
  return { lastCreatedAt: Number.isFinite(createdAt) ? createdAt : 0, lastId: comment.id + 1 };
}

async function locateComment(commentId: number) {
  highlightId.value = 0;
  let target: Comment;
  try {
    target = await fetchComment(commentId);
  } catch {
    await loadFirstPage();
    showToast("这条评论已经不存在");
    return;
  }

  if (target.parentId === 0) {    const page = await fetchCommentList(props.videoId, cursorBefore(target));
    comments.value = page.comments;
    cursor.value = page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null;
    hasMore.value = page.hasMore;
    replyStates.value = {};
    await reveal(commentId);
    return;
  }

  // 回复：先把父评论取出来放进列表，再展开这个楼并把目标那条回复放在最前
  let parent: Comment;
  try {
    parent = await fetchComment(target.parentId);
  } catch {
    await loadFirstPage();
    showToast("这条评论已经不存在");
    return;
  }
  const page = await fetchCommentList(props.videoId, cursorBefore(parent));
  comments.value = page.comments;
  cursor.value = page.lastId ? { lastCreatedAt: page.lastCreatedAt, lastId: page.lastId } : null;
  hasMore.value = page.hasMore;

  const replies = await fetchReplyList(parent.id, { limit: firstExpandSize, cursor: cursorBefore(target) });
  replyStates.value = {
    [parent.id]: {
      open: true,
      replies: replies.comments,
      cursor: replies.lastId ? { lastCreatedAt: replies.lastCreatedAt, lastId: replies.lastId } : null,
      hasMore: replies.hasMore
    }
  };
  await reveal(commentId);
}

// reveal 把定位到的那条滚动到可见位置并高亮，几秒后取消高亮
async function reveal(commentId: number) {
  highlightId.value = commentId;
  await nextTick();
  const node = document.querySelector(`[data-comment-id="${commentId}"]`);
  node?.scrollIntoView({ block: "center" });
  window.setTimeout(() => {
    if (highlightId.value === commentId) {
      highlightId.value = 0;
    }
  }, 3000);
}

watch(
  () => [props.open, props.videoId, props.focusCommentId ?? 0] as const,
  async ([isOpen, videoId]) => {
    if (!isOpen || !videoId) return;
    replyTarget.value = null;
    try {
      const me = await fetchMe();
      currentUserId.value = me.id;
    } catch {
      currentUserId.value = null;
    }
    if (props.focusCommentId) {
      await locateComment(props.focusCommentId);
      return;
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
  background-color: rgba(0, 0, 0, 0.65);
  z-index: 90;
  backdrop-filter: blur(4px);
}

.sheet {
  position: absolute;
  top: 0;
  right: 0;
  width: min(440px, 36vw);
  height: 100svh;
  background-color: rgba(18, 18, 18, 0.98);
  border-left: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: -20px 0 60px rgba(0, 0, 0, 0.8);
  display: flex;
  flex-direction: column;
}

header {
  padding: 16px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #ffffff;
}

.close-btn {
  border: none;
  background-color: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.7);
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 14px;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.close-btn:hover {
  background-color: rgba(255, 255, 255, 0.16);
  color: #ffffff;
}

.list {
  flex: 1;
  margin: 0;
  padding: 8px 20px;
  overflow-y: auto;
  list-style: none;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.list::-webkit-scrollbar {
  display: none;
}

li {
  padding: 14px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

/* 从通知跳转定位到的那条评论 */
li.focused {
  background-color: rgba(254, 44, 85, 0.12);
  border-left: 3px solid var(--tiktok-red);
  border-radius: 6px;
  padding-left: 10px;
}

.author-row {
  display: flex;
  align-items: center;
  gap: 10px;
  color: rgba(255, 255, 255, 0.65);
  font-size: 13px;
  font-weight: 600;
}

.author-row img {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
  border: 1px solid rgba(255, 255, 255, 0.15);
}

p {
  margin: 8px 0;
  color: #ffffff;
  font-size: 14px;
  line-height: 1.45;
  word-break: break-word;
}

small {
  display: flex;
  align-items: center;
  gap: 16px;
  color: rgba(255, 255, 255, 0.45);
  font-size: 12px;
}

.like-btn,
.reply-btn,
.delete-btn {
  border: none;
  color: rgba(255, 255, 255, 0.55);
  background: transparent;
  padding: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  font-weight: 600;
  transition: color 0.15s ease;
}

.like-btn:hover,
.reply-btn:hover {
  color: #ffffff;
}

.like-btn.active {
  color: var(--tiktok-red);
}

.delete-btn {
  color: rgba(255, 255, 255, 0.35);
}

.delete-btn:hover {
  color: var(--tiktok-red);
}

.me-badge {
  background-color: var(--tiktok-red);
  color: #ffffff;
  font-style: normal;
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
  padding: 2px 6px;
  border-radius: 999px;
}

.icon-svg {
  width: 14px;
  height: 14px;
  fill: currentColor;
}

.replies {
  margin: 10px 0 0;
  padding: 10px 0 0 16px;
  border-left: 2px solid rgba(255, 255, 255, 0.08);
  list-style: none;
}

.replies li {
  padding: 8px 0;
  border-bottom: none;
}

.reply-author {
  color: rgba(255, 255, 255, 0.7);
  font-size: 12px;
  font-weight: 600;
}

.reply-author img {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  object-fit: cover;
  vertical-align: middle;
  margin-right: 6px;
}

.reply-to {
  color: rgba(255, 255, 255, 0.45);
  font-size: 12px;
  margin-left: 6px;
}

.replies p {
  margin: 4px 0 6px;
  font-size: 13px;
}

.more-btn,
.load-btn {
  border: none;
  background: transparent;
  color: rgba(255, 255, 255, 0.65);
  font-size: 13px;
  font-weight: 600;
  padding: 8px 0;
  display: block;
  cursor: pointer;
  transition: color 0.15s ease;
}

.more-btn:hover,
.load-btn:hover {
  color: #ffffff;
}

.load-btn {
  margin: 8px 20px;
  text-align: center;
  background-color: rgba(255, 255, 255, 0.06);
  border-radius: 8px;
  padding: 10px 0;
}

.composer {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 8px;
  padding: 14px 20px calc(14px + var(--safe-bottom));
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  background-color: #121212;
}

input {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  background-color: rgba(255, 255, 255, 0.07);
  color: #ffffff;
  padding: 10px 16px;
  outline: none;
  transition: border-color 0.18s ease;
}

input:focus {
  border-color: rgba(255, 255, 255, 0.35);
}

.composer button {
  border: none;
  border-radius: 999px;
  background-color: var(--tiktok-red);
  color: #ffffff;
  padding: 0 18px;
  font-size: 14px;
  font-weight: 600;
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.composer button:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.composer .cancel-reply {
  background-color: transparent;
  color: rgba(255, 255, 255, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.15);
}

.drawer-enter-active,
.drawer-leave-active {
  transition: opacity 0.22s ease;
}

.drawer-enter-from,
.drawer-leave-to {
  opacity: 0;
}

.drawer-enter-active .sheet,
.drawer-leave-active .sheet {
  transition: transform 0.26s cubic-bezier(0.16, 1, 0.3, 1);
}

.drawer-enter-from .sheet,
.drawer-leave-to .sheet {
  transform: translateX(100%);
}

@media (max-width: 768px) {
  .sheet {
    width: 100vw;
    height: 72svh;
    top: auto;
    bottom: 0;
    border-radius: 16px 16px 0 0;
    border-left: none;
    border-top: 1px solid rgba(255, 255, 255, 0.12);
  }

  .drawer-enter-from .sheet,
  .drawer-leave-to .sheet {
    transform: translateY(100%);
  }
}
</style>
