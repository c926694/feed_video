import { http } from "@/utils/http";
import { unwrapData } from "@/utils/normalize";
import type { MessageActor, MessageItem, MessagePage } from "@/types/domain";

export interface MessageCursor {
  lastCreatedAt?: number;
  lastId?: number;
  limit?: number;
}

function toNumber(value: unknown, fallback = 0) {
  const n = Number(value);
  return Number.isFinite(n) ? n : fallback;
}

function toString(value: unknown, fallback = "") {
  return typeof value === "string" ? value : fallback;
}

function normalizeActor(raw: unknown): MessageActor {
  const payload = (raw ?? {}) as Record<string, unknown>;
  return {
    id: toNumber(payload.id),
    nickname: toString(payload.nickname, "匿名用户"),
    avatar: toString(payload.avatar_URL ?? payload.avatar_url)
  };
}

function normalizeMessage(raw: unknown): MessageItem {
  const payload = (raw ?? {}) as Record<string, unknown>;
  const actors = Array.isArray(payload.actors) ? payload.actors.map(normalizeActor) : [];
  return {
    id: toNumber(payload.id),
    type: toString(payload.type),
    title: toString(payload.title, "收到消息"),
    content: toString(payload.content),
    actors,
    actorCount: toNumber(payload.actor_count, actors.length),
    videoId: toNumber(payload.video_id),
    commentId: toNumber(payload.comment_id),
    coverUrl: toString(payload.coverURL ?? payload.cover_url),
    isRead: Boolean(payload.is_read),
    createdAt: toString(payload.created_at)
  };
}

// fetchMessageList 取通知列表：首页不传游标，下一页把上一页返回的两个游标原样传回
export async function fetchMessageList(cursor: MessageCursor = {}): Promise<MessagePage> {
  const { data } = await http.get("/messages", {
    params: {
      ...(cursor.limit ? { limit: cursor.limit } : {}),
      ...(cursor.lastId ? { last_created_at: cursor.lastCreatedAt, last_id: cursor.lastId } : {})
    }
  });
  const body = unwrapData<unknown>(data);
  const payload = typeof body === "object" && body ? (body as Record<string, unknown>) : {};
  const list = Array.isArray(payload.message_list) ? payload.message_list : [];
  return {
    messages: list.map(normalizeMessage),
    lastCreatedAt: toNumber(payload.last_created_at),
    lastId: toNumber(payload.last_id)
  };
}

// fetchUnreadCount 取未读会话数，供底部导航的徽标使用
export async function fetchUnreadCount(): Promise<number> {
  const { data } = await http.get("/messages/unread");
  const body = unwrapData<unknown>(data);
  if (!body || typeof body !== "object") return 0;
  return toNumber((body as Record<string, unknown>).unread_count);
}

// markMessagesRead 标记已读，ids 为空数组表示全部已读
export async function markMessagesRead(ids: number[] = []): Promise<number> {
  const { data } = await http.post("/messages/read", { message_ids: ids });
  const body = unwrapData<unknown>(data);
  if (!body || typeof body !== "object") return 0;
  return toNumber((body as Record<string, unknown>).updated);
}
