import { http } from "@/utils/http";
import { normalizeVideo, unwrapData } from "@/utils/normalize";
import type { RawVideo } from "@/types/backend";
import type { Video } from "@/types/domain";

interface FeedParams {
  limit?: number;
  lastCreatedAt?: number;
  lastId?: number;
}

interface HotFeedParams {
  limit?: number;
  offset?: number;
  interval?: number;
}

function pickVideoList(source: unknown): RawVideo[] {
  if (Array.isArray(source)) return source as RawVideo[];
  if (!source || typeof source !== "object") return [];
  const payload = source as Record<string, unknown>;
  const candidates = [payload.list, payload.items, payload.videos, payload.video_list, payload.feed_video_list];
  const found = candidates.find((entry) => Array.isArray(entry));
  return (found as RawVideo[] | undefined) ?? [];
}

export async function fetchFeedVideos(params: FeedParams = {}) {
  return fetchFeedByPath("/videos/feed", params);
}

export async function fetchHotVideos(params: HotFeedParams = {}) {
  const { limit = 5, offset = 0, interval = 60 } = params;
  const { data } = await http.get("/videos/feed/hot", {
    params: {
      limit,
      offset,
      interval,
      _ts: Date.now()
    }
  });
  const body = unwrapData<unknown>(data);
  const list = pickVideoList(body);
  const payload = typeof body === "object" && body ? (body as Record<string, unknown>) : {};
  const nextOffset = Number(payload.next_offset ?? offset + list.length);
  return {
    videos: list.map(normalizeVideo),
    nextOffset: Number.isFinite(nextOffset) ? nextOffset : offset + list.length,
    hasMore: Boolean(payload.has_more),
    interval: Number(payload.interval ?? interval)
  };
}

export async function fetchFollowVideos(params: FeedParams = {}) {
  return fetchFeedByPath("/videos/feed/follow", params);
}

export async function fetchMyVideos(limit = 60) {
  const { data } = await http.get("/videos/me", {
    params: { limit }
  });
  const body = unwrapData<unknown>(data);
  return pickVideoList(body).map(normalizeVideo);
}

async function fetchFeedByPath(path: string, params: FeedParams = {}) {
  const { limit = 5, lastCreatedAt, lastId } = params;
  const { data } = await http.get(path, {
    params: {
      limit,
      _ts: Date.now(),
      ...(lastId ? { last_created_at: lastCreatedAt, last_id: lastId } : {})
    }
  });
  const body = unwrapData<unknown>(data);
  const list = pickVideoList(body);
  const payload = typeof body === "object" && body ? (body as Record<string, unknown>) : {};
  return {
    videos: list.map(normalizeVideo),
    nextCreatedAt: Number(payload.last_created_at ?? 0),
    nextId: Number(payload.last_id ?? 0)
  };
}

export async function createVideo(payload: { title: string; description: string; cover: File; play: File }) {
  const formData = new FormData();
  formData.append("title", payload.title);
  formData.append("description", payload.description);
  formData.append("cover", payload.cover);
  formData.append("play", payload.play);
  await http.post("/videos/create", formData);
}

export async function deleteVideo(videoId: number) {
  await http.delete(`/videos/${videoId}`);
}

export function filterVideosByUser(videos: Video[], userId: number) {
  return videos.filter((video) => video.author.id === userId);
}
