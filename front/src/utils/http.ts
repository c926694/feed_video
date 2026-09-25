import axios, { type AxiosError } from "axios";
import { getToken } from "@/utils/storage";
import { useAuth } from "@/composables/useAuth";
import { useToast } from "@/composables/useToast";
import type { ApiEnvelope } from "@/types/backend";

const { showToast } = useToast();
const { clearAuth } = useAuth();

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

http.interceptors.response.use(
  (response) => {
    // 业务错误统一通过响应体里的 code 判定，msg 直接展示给用户
    const envelope = response.data as ApiEnvelope;
    if (typeof envelope?.code === "number" && envelope.code !== 0) {
      const msg = envelope.msg ?? "请求失败";
      showToast(msg);
      return Promise.reject(new Error(msg));
    }
    return response;
  },
  (error: AxiosError<ApiEnvelope>) => {
    const status = error.response?.status;
    const msg = error.response?.data?.msg;
    if (status === 401) {
      clearAuth();
      showToast(msg ?? "登录已失效，请重新登录");
      if (window.location.pathname !== "/login") {
        window.location.href = "/login";
      }
    } else {
      showToast(msg ?? error.message);
    }
    return Promise.reject(error);
  }
);
