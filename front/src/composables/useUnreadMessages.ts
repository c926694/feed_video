import { onBeforeUnmount, onMounted, ref } from "vue";
import { fetchUnreadCount } from "@/api";
import { getToken } from "@/utils/storage";

// 未读消息数在多个组件里展示（首页左栏、底部导航、消息页），
// 用模块级的 ref 与一个共享的轮询定时器，避免每处各拉一次
const unreadCount = ref(0);
const pollInterval = 60_000;

let timer: number | null = null;
let subscribers = 0;

async function refresh() {
  if (!getToken()) {
    unreadCount.value = 0;
    return;
  }
  try {
    unreadCount.value = await fetchUnreadCount();
  } catch {
    // 拉不到就保持上一次的值，不清零
  }
}

export function useUnreadMessages() {
  onMounted(() => {
    subscribers += 1;
    if (timer === null) {
      void refresh();
      timer = window.setInterval(() => {
        void refresh();
      }, pollInterval);
    }
  });

  onBeforeUnmount(() => {
    subscribers -= 1;
    if (subscribers <= 0 && timer !== null) {
      window.clearInterval(timer);
      timer = null;
    }
  });

  return { unreadCount, refresh };
}
