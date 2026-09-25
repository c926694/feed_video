<template>
  <section class="auth-page">
    <form class="panel" @submit.prevent="handleRegister">
      <h1>创建账号</h1>
      <label>
        用户名
        <input v-model.trim="form.username" required />
      </label>
      <label>
        密码
        <input v-model.trim="form.password" type="password" required />
      </label>
      <label>
        确认密码
        <input v-model.trim="form.re_password" type="password" required />
      </label>

      <button :disabled="loading" type="submit">{{ loading ? "提交中..." : "注册" }}</button>
      <RouterLink to="/login" class="link">返回登录</RouterLink>
    </form>
  </section>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { registerUser } from "@/api";
import { useToast } from "@/composables/useToast";

const router = useRouter();
const { showToast } = useToast();

const form = reactive({
  username: "",
  password: "",
  re_password: ""
});
const loading = ref(false);

async function handleRegister() {
  if (form.password !== form.re_password) {
    showToast("两次输入的密码不一致");
    return;
  }
  loading.value = true;
  try {
    await registerUser(form);
    showToast("注册成功");
    router.push("/login");
  } catch {
    // 错误提示已由 http 拦截器统一弹出
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.auth-page {
  min-height: 100svh;
  display: grid;
  place-items: center;
  padding: 20px;
}

.panel {
  width: min(420px, 92vw);
  margin: 0 auto;
  padding: 28px 22px;
  border-radius: 20px;
  background: rgba(9, 12, 20, 0.8);
  border: 1px solid var(--line);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

h1 {
  margin: 0;
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
}

button {
  margin-top: 4px;
  border: none;
  border-radius: 999px;
  background: linear-gradient(90deg, #18b6ff, #42d77d);
  color: #fff;
  padding: 12px;
  font-weight: 700;
}

.link {
  text-align: center;
  font-size: 13px;
  color: var(--accent-alt);
}
</style>
