import { computed, ref } from "vue";
import {
  clearRefreshToken,
  clearToken,
  getRefreshToken,
  getToken,
  setRefreshToken,
  setToken
} from "@/utils/storage";
import type { TokenPair, User } from "@/types/domain";

const token = ref(getToken());
const refreshToken = ref(getRefreshToken());
const currentUser = ref<User | null>(null);

export function useAuth() {
  const isLoggedIn = computed(() => Boolean(token.value));

  const setAuth = (pair: TokenPair) => {
    token.value = pair.accessToken;
    refreshToken.value = pair.refreshToken;
    setToken(pair.accessToken);
    setRefreshToken(pair.refreshToken);
  };

  const clearAuth = () => {
    token.value = "";
    refreshToken.value = "";
    currentUser.value = null;
    clearToken();
    clearRefreshToken();
  };

  return {
    token,
    refreshToken,
    currentUser,
    isLoggedIn,
    setAuth,
    clearAuth
  };
}
