<template>
  <nav class="bottom-nav">
    <RouterLink class="nav-item" :class="{ active: route.path === '/feed' }" to="/feed">
      <span class="icon-wrap">
        <svg viewBox="0 0 24 24" class="nav-svg">
          <path d="M12 3 3 10.5V21h6v-6h6v6h6V10.5L12 3Z" fill="currentColor" />
        </svg>
      </span>
      <span class="nav-label">首页</span>
    </RouterLink>

    <RouterLink class="nav-item" :class="{ active: route.path === '/messages' }" to="/messages">
      <span class="icon-wrap">
        <svg viewBox="0 0 24 24" class="nav-svg">
          <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2Zm8 7L4 6v2l8 5 8-5V6l-8 5Z" fill="currentColor" />
        </svg>
        <span v-if="unreadCount > 0" class="nav-badge">{{ badgeText }}</span>
      </span>
      <span class="nav-label">消息</span>
    </RouterLink>

    <RouterLink class="nav-item create-item" to="/upload" title="发布视频">
      <div class="tiktok-create-pill" aria-hidden="true">
        <span class="pill-cyan"></span>
        <span class="pill-red"></span>
        <span class="pill-white">
          <svg viewBox="0 0 24 24" class="create-plus-svg">
            <path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2Z" fill="#000000" />
          </svg>
        </span>
      </div>
    </RouterLink>

    <RouterLink class="nav-item" :class="{ active: route.path === '/profile' }" to="/profile">
      <span class="icon-wrap">
        <svg viewBox="0 0 24 24" class="nav-svg">
          <path
            d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4Zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4Z"
            fill="currentColor"
          />
        </svg>
      </span>
      <span class="nav-label">我的</span>
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { useUnreadMessages } from "@/composables/useUnreadMessages";

const route = useRoute();
const { unreadCount } = useUnreadMessages();
const badgeText = computed(() => (unreadCount.value > 99 ? "99+" : String(unreadCount.value)));
</script>

<style scoped>
.bottom-nav {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 80;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  align-items: center;
  background-color: #000000;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding: 8px 16px calc(8px + var(--safe-bottom));
}

.nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3px;
  color: rgba(255, 255, 255, 0.6);
  transition: color 0.15s ease;
}

.nav-item.active {
  color: #ffffff;
}

.icon-wrap {
  position: relative;
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
}

.nav-badge {
  position: absolute;
  top: -6px;
  right: -8px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 999px;
  background-color: var(--tiktok-red);
  color: #ffffff;
  font-size: 10px;
  font-weight: 700;
  line-height: 16px;
  text-align: center;
}

.nav-svg {
  width: 22px;
  height: 22px;
}

.nav-label {
  font-size: 11px;
  font-weight: 600;
}

.create-item {
  justify-content: center;
}

.tiktok-create-pill {
  position: relative;
  width: 44px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.pill-cyan,
.pill-red,
.pill-white {
  position: absolute;
  top: 0;
  bottom: 0;
  border-radius: 8px;
}

.pill-cyan {
  left: 0;
  right: 6px;
  background-color: var(--tiktok-cyan);
}

.pill-red {
  left: 6px;
  right: 0;
  background-color: var(--tiktok-red);
}

.pill-white {
  left: 3px;
  right: 3px;
  background-color: #ffffff;
  display: grid;
  place-items: center;
  z-index: 2;
}

.create-plus-svg {
  width: 18px;
  height: 18px;
}
</style>
