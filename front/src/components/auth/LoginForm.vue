<template>
  <form class="panel" @submit.prevent="handleSubmit">
    <h1>Feed Video</h1>
    <p class="sub">沉浸式短视频体验，从登录开始。</p>

    <label>
      用户名
      <input v-model.trim="form.username" required placeholder="请输入用户名" />
    </label>

    <label>
      密码
      <input v-model.trim="form.password" type="password" required placeholder="请输入密码" />
    </label>

    <button :disabled="loading" type="submit">
      {{ loading ? "登录中..." : "登录" }}
    </button>

    <RouterLink to="/register" class="link">没有账号？去注册</RouterLink>
  </form>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { login } from "@/api";
import { useAuth } from "@/composables/useAuth";
import { useToast } from "@/composables/useToast";

const router = useRouter();
const { setAuthToken } = useAuth();
const { showToast } = useToast();

const loading = ref(false);
const form = reactive({
  username: "",
  password: ""
});

async function handleSubmit() {
  loading.value = true;
  try {
    const result = await login(form);
    if (!result.token) {
      showToast("登录成功但未拿到 token，请检查后端返回字段。");
      return;
    }
    setAuthToken(result.token);
    showToast("登录成功");
    router.push("/feed");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.panel {
  width: min(420px, 92vw);
  margin: 0 auto;
  padding: 28px 22px;
  border-radius: 20px;
  background: rgba(9, 12, 20, 0.8);
  border: 1px solid var(--line);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

h1 {
  margin: 0;
  font-size: 32px;
  letter-spacing: 0.02em;
}

.sub {
  margin: 0;
  color: var(--text-muted);
  font-size: 14px;
}

label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  color: var(--text-muted);
}

input {
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.04);
  border-radius: 12px;
  color: var(--text-primary);
  padding: 11px 12px;
  outline: none;
}

input:focus {
  border-color: rgba(255, 77, 109, 0.7);
}

button {
  margin-top: 4px;
  border: none;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--accent), #ff8755);
  color: #fff;
  padding: 12px;
  font-weight: 700;
  cursor: pointer;
}

button:disabled {
  opacity: 0.6;
}

.link {
  text-align: center;
  font-size: 13px;
  color: var(--accent-alt);
}
</style>
