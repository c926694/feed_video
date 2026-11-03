import { ref } from "vue";

const globalMuted = ref(false);

export function useSoundSetting() {
  function setMuted(val: boolean) {
    globalMuted.value = val;
  }

  function toggleMuted() {
    globalMuted.value = !globalMuted.value;
  }

  return {
    globalMuted,
    setMuted,
    toggleMuted
  };
}
