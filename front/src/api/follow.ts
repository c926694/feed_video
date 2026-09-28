import { http } from "@/utils/http";
import { unwrapData } from "@/utils/normalize";

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
