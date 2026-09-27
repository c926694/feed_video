import axios, { type AxiosError, type InternalAxiosRequestConfig } from "axios";
import { getRefreshToken, getToken } from "@/utils/storage";
import { useAuth } from "@/composables/useAuth";
import { useToast } from "@/composables/useToast";
import type { ApiEnvelope } from "@/types/backend";

const { showToast } = useToast();
const { clearAuth, setAuth } = useAuth();

export const http = axios.create({
  baseURL: "/api",
  timeout: 12000
});

http.interceptors.request.use((config) => {
  const token = getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// runRefresh 用 refresh token 换一对新令牌。这里用裸 axios 绕开本实例的拦截器，避免递归。
async function runRefresh(): Promise<boolean> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return false;
  try {
    const { data } = await axios.post("/api/users/refresh", { refresh_token: refreshToken });
    const envelope = data as ApiEnvelope<unknown>;
    const body = (envelope?.data ?? {}) as Record<string, unknown>;
    const accessToken = String(body.access_token ?? "");
    const nextRefreshToken = String(body.refresh_token ?? "");
    if (!accessToken || !nextRefreshToken) return false;
    setAuth({ accessToken, refreshToken: nextRefreshToken });
    return true;
  } catch {
    return false;
  }
}

// 并发请求同时 401 时共享同一次刷新，避免旧 refresh token 被重复使用
let refreshPromise: Promise<boolean> | null = null;

function refreshOnce(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = runRefresh().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

function redirectToLogin(msg: string) {
  clearAuth();
  showToast(msg);
  if (window.location.pathname !== "/login") {
    window.location.href = "/login";
  }
}

http.interceptors.response.use(
  (response) => {
    // 业务错误统一通过响应体里的 code 判定，msg 直接展示给用户
    const envelope = response.data as ApiEnvelope;
    if (typeof envelope?.code === "number" && envelope.code !== 0) {
      const msg = envelope.msg ?? "请求失败";
      if (envelope.code !== 40900) {
        showToast(msg);
      }
      const error = new Error(msg) as Error & { envelope?: ApiEnvelope };
      error.envelope = envelope;
      return Promise.reject(error);
    }
    return response;
  },
  async (error: AxiosError<ApiEnvelope>) => {
    const status = error.response?.status;
    const config = error.config as (InternalAxiosRequestConfig & { retried?: boolean }) | undefined;
    // access token 过期：先刷新再重放原请求，刷新失败才登出
    if (status === 401 && config && !config.retried && !config.url?.includes("/users/refresh")) {
      config.retried = true;
      if (await refreshOnce()) {
        return http.request(config);
      }
      redirectToLogin(error.response?.data?.msg ?? "登录已失效，请重新登录");
      return Promise.reject(error);
    }
    if (status === 401) {
      redirectToLogin(error.response?.data?.msg ?? "登录已失效，请重新登录");
      return Promise.reject(error);
    }
    showToast(error.response?.data?.msg ?? error.message);
    return Promise.reject(error);
  }
);
