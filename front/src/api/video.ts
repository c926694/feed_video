import { http } from "@/utils/http";
import { normalizeVideo, pickVideoList, unwrapData } from "@/utils/normalize";
import type { ApiEnvelope } from "@/types/backend";
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

// 直传凭证
export interface UploadCredential {
  accessKeyId: string;
  accessKeySecret: string;
  securityToken: string;
  expiration: number;
  region: string;
  bucket: string;
  coverKey: string;
  playKey: string;
}

function credentialOf(body: unknown): UploadCredential {
  const payload = typeof body === "object" && body ? (body as Record<string, unknown>) : {};
  return {
    accessKeyId: String(payload.access_key_id ?? ""),
    accessKeySecret: String(payload.access_key_secret ?? ""),
    securityToken: String(payload.security_token ?? ""),
    expiration: Number(payload.expiration ?? 0),
    region: String(payload.region ?? ""),
    bucket: String(payload.bucket ?? ""),
    coverKey: String(payload.cover_key ?? ""),
    playKey: String(payload.play_key ?? "")
  };
}

export async function fetchFeedVideos(params: FeedParams = {}) {
  return fetchFeedByPath("/videos/feed", params);
}

// fetchAuthorVideos 取指定作者的已发布视频，看别人的主页时使用
export async function fetchAuthorVideos(authorId: number, params: FeedParams = {}) {
  const { data } = await http.get(`/videos/author/${authorId}`, {
    params: {
      ...(params.limit ? { limit: params.limit } : {}),
      ...(params.lastId ? { last_created_at: params.lastCreatedAt, last_id: params.lastId } : {})
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

// fetchVideoDetail 取单条视频，含当前登录用户的点赞、收藏与关注状态
export async function fetchVideoDetail(videoId: number): Promise<Video> {
  const { data } = await http.get(`/videos/${videoId}`);
  return normalizeVideo(unwrapData<unknown>(data));
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

// 拿直传凭证，后端生成封面与视频的存储路径
export async function fetchUploadCredential(payload: { coverExt: string; playExt: string }): Promise<UploadCredential> {
  const { data } = await http.post("/videos/upload-credential", {
    cover_ext: payload.coverExt,
    play_ext: payload.playExt
  });
  return credentialOf(unwrapData<unknown>(data));
}

// 创建发布记录，状态为已创建。重复提交时返回已创建的记录，流程可以继续。
export async function createVideo(payload: { title: string; description: string; coverKey: string; playKey: string; requestId: string }) {
  try {
    const { data } = await http.post("/videos", {
      title: payload.title,
      description: payload.description,
      cover_key: payload.coverKey,
      play_key: payload.playKey,
      request_id: payload.requestId
    });
    const body = unwrapData<{ id: number; status: string }>(data);
    return { id: body.id, status: body.status };
  } catch (error) {
    const envelope = (error as { envelope?: ApiEnvelope }).envelope;
    if (envelope?.code === 40900 && envelope.data) {
      // 重复创建：把已创建的记录当作创建结果继续
      const body = envelope.data as { id: number; status: string };
      return { id: body.id, status: body.status };
    }
    throw error;
  }
}

// updateVideoStatus：published 发布完成、failed 标记失败、created 重试，
// private 设为私密，published 用在私密视频上表示取消私密。
// created 时返回该视频的存储路径与新凭证，其余返回 null。
export async function updateVideoStatus(videoId: number, status: "published" | "failed" | "created" | "private"): Promise<UploadCredential | null> {
  const { data } = await http.put(`/videos/${videoId}`, { status });
  const body = unwrapData<unknown>(data);
  if (!body) return null;
  return credentialOf(body);
}

export async function deleteVideo(videoId: number) {
  await http.delete(`/videos/${videoId}`);
}

// setVideoPrivate 设置视频是否只对自己可见：隐藏发 private，取消隐藏发 published
export async function setVideoPrivate(videoId: number, isPrivate: boolean) {
  await updateVideoStatus(videoId, isPrivate ? "private" : "published");
}

export function filterVideosByUser(videos: Video[], userId: number) {
  return videos.filter((video) => video.author.id === userId);
}
