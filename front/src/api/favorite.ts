import { http } from "@/utils/http";
import { unwrapData } from "@/utils/normalize";
import { fetchVideoPage, type VideoCursor } from "./videoList";
import type { VideoPage } from "@/types/domain";

// 显式目标态：active 为 true 发 POST 收藏，false 发 DELETE 取消。
// 重复请求同一目标态是幂等空操作
export async function setVideoFavorite(videoId: number, active: boolean) {
  const { data } = active
    ? await http.post(`/favorites/video/${videoId}`)
    : await http.delete(`/favorites/video/${videoId}`);
  const body = unwrapData<unknown>(data);
  if (!body || typeof body !== "object") return false;
  const payload = body as Record<string, unknown>;
  return Boolean(payload.is_favorited ?? payload.isFavorite);
}

export function fetchMyFavorites(cursor: VideoCursor = {}): Promise<VideoPage> {
  return fetchVideoPage("/favorites/me", cursor);
}
