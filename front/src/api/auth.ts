import { http } from "@/utils/http";
import { normalizeUser, unwrapData } from "@/utils/normalize";
import type { ApiEnvelope, RawUser } from "@/types/backend";
import type { TokenPair, User } from "@/types/domain";

interface LoginPayload {
  username: string;
  password: string;
}

function toTokenPair(raw: unknown): TokenPair {
  if (!raw || typeof raw !== "object") {
    return { accessToken: "", refreshToken: "" };
  }
  const body = raw as Record<string, unknown>;
  return {
    accessToken: String(body.access_token ?? body.token ?? body.jwt ?? ""),
    refreshToken: String(body.refresh_token ?? "")
  };
}

export async function login(payload: LoginPayload): Promise<TokenPair> {
  const { data } = await http.post("/users/login", payload);
  return toTokenPair(unwrapData<unknown>(data));
}

// refreshTokens 用 refresh token 换一对新令牌
export async function refreshTokens(refreshToken: string): Promise<TokenPair> {
  const { data } = await http.post("/users/refresh", { refresh_token: refreshToken });
  return toTokenPair(unwrapData<unknown>(data));
}

export async function fetchMe() {
  const { data } = await http.get("/users/me");
  const body = unwrapData<RawUser | { user?: RawUser }>(data);
  if ("user" in (body as { user?: RawUser })) {
    return normalizeUser((body as { user?: RawUser }).user);
  }
  return normalizeUser(body as RawUser);
}

// fetchUserProfile 取指定用户的公开资料，看别人的主页时使用
export async function fetchUserProfile(userId: number): Promise<User> {
  const { data } = await http.get(`/users/${userId}`);
  const body = unwrapData<RawUser | { user?: RawUser }>(data);
  if ("user" in (body as { user?: RawUser })) {
    return normalizeUser((body as { user?: RawUser }).user);
  }
  return normalizeUser(body as RawUser);
}

export async function updateMyProfile(payload: { nickname?: string; avatar?: File | null }) {
  const formData = new FormData();
  if (payload.nickname && payload.nickname.trim()) {
    formData.append("nickname", payload.nickname.trim());
  }
  if (payload.avatar) {
    formData.append("avatar", payload.avatar);
  }
  const { data } = await http.post("/users/me", formData);
  const body = unwrapData<RawUser | { user?: RawUser }>(data);
  if ("user" in (body as { user?: RawUser })) {
    return normalizeUser((body as { user?: RawUser }).user);
  }
  return normalizeUser(body as RawUser);
}

// logout 提交 refresh token，服务端删除这条记录
export async function logout(refreshToken: string) {
  await http.delete("/users/logout", { data: { refresh_token: refreshToken } });
}

export async function registerUser(payload: { username: string; password: string; re_password: string }) {
  const { data } = await http.post("/users/register", payload);
  const body = unwrapData<unknown>(data);
  return body;
}

export function pickUserFromRaw(raw: Record<string, unknown>): User | null {
  const maybeUser = raw.user as RawUser | undefined;
  if (maybeUser) return normalizeUser(maybeUser);
  return null;
}
