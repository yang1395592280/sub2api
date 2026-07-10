import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { getZenxiangLiyuStatus, type ZenxiangLiyuStatus } from '@/api/zenxiangLiyu'

export const useZenxiangLiyuStore = defineStore('zenxiangLiyu', () => {
  const status = ref<ZenxiangLiyuStatus | null>(null)
  const loading = ref(false)
  const loaded = ref(false)

  const visible = computed(() => status.value?.visible === true)
  const canPlay = computed(() => status.value?.can_play === true)

  async function loadStatus(): Promise<void> {
    if (loading.value) return

    loading.value = true
    try {
      status.value = await getZenxiangLiyuStatus()
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  return { status, loading, loaded, visible, canPlay, loadStatus }
})
