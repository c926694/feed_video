import { http } from "@/utils/http";
import { normalizeVideo, pickVideoList, unwrapData } from "@/utils/normalize";
import type { VideoPage } from "@/types/domain";

export interface VideoCursor {
  lastCreatedAt?: number;
  lastId?: number;
  limit?: number;
}

const pageSize = 20;
const maxPages = 5;

// fetchVideoPage 取带双字段游标的视频列表：首页不传游标，
// 下一页把上一页返回的 lastCreatedAt 与 lastId 原样传回
export async function fetchVideoPage(path: string, cursor: VideoCursor = {}): Promise<VideoPage> {
  const { data } = await http.get(path, {
    params: {
      ...(cursor.limit ? { limit: cursor.limit } : {}),
      ...(cursor.lastId ? { last_created_at: cursor.lastCreatedAt, last_id: cursor.lastId } : {})
    }
  });
  const body = unwrapData<unknown>(data);
  const payload = typeof body === "object" && body ? (body as Record<string, unknown>) : {};
  return {
    videos: pickVideoList(body).map(normalizeVideo),
    lastCreatedAt: Number(payload.last_created_at ?? 0),
    lastId: Number(payload.last_id ?? 0),
    hasMore: Boolean(payload.has_more)
  };
}

// fetchAllVideoPages 逐页取到没有下一页为止，最多取 maxPages 页
export async function fetchAllVideoPages(fetchPage: (cursor: VideoCursor) => Promise<VideoPage>): Promise<Video[]> {
  const all: Video[] = [];
  let cursor: VideoCursor = { limit: pageSize };
  for (let page = 0; page < maxPages; page += 1) {
    const result = await fetchPage(cursor);
    all.push(...result.videos);
    if (!result.hasMore || !result.lastId) break;
    cursor = { limit: pageSize, lastCreatedAt: result.lastCreatedAt, lastId: result.lastId };
  }
  return all;
}
