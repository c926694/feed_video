import { http } from "@/utils/http";
import { unwrapData } from "@/utils/normalize";
import { fetchVideoPage, type VideoCursor } from "./videoList";
import type { VideoPage } from "@/types/domain";

function pickIsLiked(payload: unknown) {
  const body = unwrapData<unknown>(payload);
  if (!body || typeof body !== "object") return false;
  const data = body as Record<string, unknown>;
  return Boolean(data.is_liked ?? data.isLiked);
}

// 显式目标态：active 为 true 发 POST 点赞，false 发 DELETE 取消。
// 重复请求同一目标态是幂等空操作
export async function setVideoLike(videoId: number, active: boolean) {
  const { data } = active
    ? await http.post(`/likes/video/${videoId}`)
    : await http.delete(`/likes/video/${videoId}`);
  return pickIsLiked(data);
}

export async function setCommentLike(commentId: number, active: boolean) {
  const { data } = active
    ? await http.post(`/likes/comment/${commentId}`)
    : await http.delete(`/likes/comment/${commentId}`);
  return pickIsLiked(data);
}

export function fetchMyLikes(cursor: VideoCursor = {}): Promise<VideoPage> {
  return fetchVideoPage("/likes/me", cursor);
}
