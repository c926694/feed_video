import { onBeforeUnmount, onMounted, ref } from "vue";

// 滚动容器接近底部时自动加载下一页。
// 整个应用是固定视口、文档不滚动，所以每个页面用自己内部的滚动容器，
// getContainer 返回那个元素；返回 null 时退化成监听窗口滚动
export function useLoadMoreOnScroll(
  getContainer: () => HTMLElement | null,
  canLoadMore: () => boolean,
  loadMore: () => Promise<void>
) {
  const loading = ref(false);
  const distance = 320;

  async function tryLoad() {
    if (loading.value || !canLoadMore()) return;
    loading.value = true;
    try {
      await loadMore();
    } finally {
      loading.value = false;
    }
  }

  function onScroll() {
    const node = getContainer();
    const scrolled = node ? node.scrollTop : window.scrollY || document.documentElement.scrollTop;
    const viewport = node ? node.clientHeight : window.innerHeight;
    const full = node ? node.scrollHeight : document.documentElement.scrollHeight;
    if (full - (scrolled + viewport) < distance) {
      void tryLoad();
    }
  }

  onMounted(() => {
    const node = getContainer();
    if (node) {
      node.addEventListener("scroll", onScroll, { passive: true });
      return;
    }
    window.addEventListener("scroll", onScroll, { passive: true });
  });

  onBeforeUnmount(() => {
    const node = getContainer();
    if (node) {
      node.removeEventListener("scroll", onScroll);
      return;
    }
    window.removeEventListener("scroll", onScroll);
  });

  return { loading, tryLoad };
}
