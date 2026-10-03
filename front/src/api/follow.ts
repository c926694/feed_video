import { http } from "@/utils/http";
import { unwrapData } from "@/utils/normalize";
import type { FollowPage, FollowUser } from "@/types/domain";

// 显式目标态：active 为 true 发 POST 关注，false 发 DELETE 取关。
// 重复请求同一目标态是幂等空操作
export async function setFollow(userId: number, active: boolean) {
  const { data } = active
    ? await http.post(`/follows/${userId}`)
    : await http.delete(`/follows/${userId}`);
  const body = unwrapData<unknown>(data);
  if (!body || typeof body !== "object") return false;
  const payload = body as Record<string, unknown>;
  return Boolean(payload.is_follow ?? payload.isFollow);
}

export interface FollowCursor {
  lastId?: number;
  limit?: number;
}

function toUser(raw: unknown): FollowUser {
  const payload = (raw ?? {}) as Record<string, unknown>;
  return {
    id: Number(payload.user_id ?? 0),
    nickname: String(payload.nickname ?? "匿名用户"),
    avatar: String(payload.avatar_URL ?? payload.avatar_url ?? ""),
    followed: Boolean(payload.is_follow)
  };
}

async function fetchFollowPage(path: string, cursor: FollowCursor = {}): Promise<FollowPage> {
  const { data } = await http.get(path, {
    params: {
      ...(cursor.limit ? { limit: cursor.limit } : {}),
      ...(cursor.lastId ? { last_id: cursor.lastId } : {})
    }
  });
  const body = unwrapData<unknown>(data);
  const payload = typeof body === "object" && body ? (body as Record<string, unknown>) : {};
  const list = Array.isArray(payload.list) ? payload.list : [];
  return {
    users: list.map(toUser),
    lastId: Number(payload.last_id ?? 0),
    hasMore: Boolean(payload.has_more)
  };
}

// fetchFollowingList 某个人关注了谁
export async function fetchFollowingList(userId: number, cursor: FollowCursor = {}) {
  return fetchFollowPage(`/follows/following/${userId}`, cursor);
}

// fetchFollowersList 谁关注了这个人
export async function fetchFollowersList(userId: number, cursor: FollowCursor = {}) {
  return fetchFollowPage(`/follows/followers/${userId}`, cursor);
}
