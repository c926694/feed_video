import { http } from "@/utils/http";
import { normalizeComment, unwrapData } from "@/utils/normalize";
import type { RawComment } from "@/types/backend";
import type { Comment } from "@/types/domain";

function pickCommentList(source: unknown): RawComment[] {
  if (Array.isArray(source)) return source as RawComment[];
  if (!source || typeof source !== "object") return [];
  const payload = source as Record<string, unknown>;
  const candidates = [payload.list, payload.items, payload.comments, payload.comment_list];
  const found = candidates.find((entry) => Array.isArray(entry));
  return (found as RawComment[] | undefined) ?? [];
}

export interface CommentPage {
  comments: Comment[];
  lastCreatedAt: number;
  lastId: number;
  hasMore: boolean;
}

export interface CommentCursor {
  lastCreatedAt?: number;
  lastId?: number;
}

function toCommentPage(body: unknown): CommentPage {
  const payload = typeof body === "object" && body ? (body as Record<string, unknown>) : {};
  return {
    comments: pickCommentList(body).map(normalizeComment),
    lastCreatedAt: Number(payload.last_created_at ?? 0),
    lastId: Number(payload.last_id ?? 0),
    hasMore: Boolean(payload.has_more)
  };
}

export async function fetchCommentList(videoId: number, cursor?: CommentCursor): Promise<CommentPage> {
  const { data } = await http.get(`/comments/list/${videoId}`, {
    params: {
      ...(cursor?.lastId ? { last_created_at: cursor.lastCreatedAt, last_id: cursor.lastId } : {})
    }
  });
  return toCommentPage(unwrapData<unknown>(data));
}

export async function fetchReplyList(
  commentId: number,
  options?: { limit?: number; cursor?: CommentCursor }
): Promise<CommentPage> {
  const { data } = await http.get(`/comments/replies/${commentId}`, {
    params: {
      ...(options?.limit ? { limit: options.limit } : {}),
      ...(options?.cursor?.lastId
        ? { last_created_at: options.cursor.lastCreatedAt, last_id: options.cursor.lastId }
        : {})
    }
  });
  return toCommentPage(unwrapData<unknown>(data));
}

// fetchComment 取单条评论，从通知跳转时用它拿到所属视频与楼层
export async function fetchComment(commentId: number): Promise<Comment> {
  const { data } = await http.get(`/comments/${commentId}`);
  return normalizeComment(unwrapData<RawComment>(data));
}

export async function createComment(payload: {
  video_id: number;
  content: string;
  parent_id?: number;
  reply_to_id?: number;
}) {
  await http.post("/comments", payload);
}

export async function deleteComment(commentId: number) {
  await http.delete(`/comments/${commentId}`);
}
