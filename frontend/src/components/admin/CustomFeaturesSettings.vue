<template>
  <section class="card space-y-6 p-6">
    <div>
      <h2 class="text-lg font-semibold">{{ t('customFeatures.title') }}</h2>
      <p class="mt-2 text-sm text-gray-500">{{ t('customFeatures.description') }}</p>
    </div>
    <p v-if="error" role="alert" class="text-sm text-red-500">{{ error }}</p>
    <div v-if="settings" class="grid gap-x-10 md:grid-cols-2">
      <label v-for="key in CUSTOM_FEATURE_KEYS" :key="key" class="flex items-center justify-between gap-4 border-b border-gray-100 py-4 dark:border-dark-700">
        <span class="text-sm">{{ t(`customFeatures.flags.${key}`) }}</span>
        <input v-model="settings.flags[key]" type="checkbox" class="h-5 w-5 accent-primary-500" :disabled="saving" />
      </label>
    </div>
    <div v-if="settings?.menus.length">
      <h3 class="mb-2 font-medium">{{ t('customFeatures.menus') }}</h3>
      <label v-for="menu in settings.menus" :key="menu.id" class="flex items-center justify-between gap-3 py-3">
        <span>{{ menu.label }}</span><input v-model="menu.enabled" type="checkbox" class="h-5 w-5 accent-primary-500" :disabled="saving" />
      </label>
    </div>
    <div class="flex items-center gap-3">
      <button type="button" class="btn btn-primary" :disabled="!settings || saving" @click="save">{{ t('common.save') }}</button>
      <span v-if="saved" role="status" class="text-sm text-green-600">{{ t('customFeatures.saved') }}</span>
    </div>
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiClient } from '@/api/client'
import { CUSTOM_FEATURE_KEYS, type CustomFeatures } from '@/api/groupMonitor'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useZenxiangLiyuStore } from '@/stores/zenxiangLiyu'
const { t } = useI18n()
const emit = defineEmits<{ saved: [settings: CustomFeatures] }>()
const settings = ref<CustomFeatures>()
const baseline = ref<CustomFeatures>()
const saving = ref(false), saved = ref(false), error = ref('')
onMounted(async () => {
  try { settings.value = (await apiClient.get<CustomFeatures>('/admin/settings/custom-features')).data; baseline.value = JSON.parse(JSON.stringify(settings.value)) }
  catch (e) { error.value = extractApiErrorMessage(e, t('common.error')) }
})
async function save() {
  if (!settings.value || !baseline.value) return
  saving.value = true; saved.value = false; error.value = ''
  const flags: Record<string, boolean> = {}, menus: Record<string, boolean> = {}
  for (const key of CUSTOM_FEATURE_KEYS) if (settings.value.flags[key] !== baseline.value.flags[key]) flags[key] = settings.value.flags[key]!
  for (const menu of settings.value.menus) if (menu.enabled !== baseline.value.menus.find(m => m.id === menu.id)?.enabled) menus[menu.id] = menu.enabled
  try {
    settings.value = (await apiClient.patch<CustomFeatures>('/admin/settings/custom-features', { flags, menus })).data
    baseline.value = JSON.parse(JSON.stringify(settings.value)); saved.value = true
    await useAppStore().fetchPublicSettings(true)
    await useAdminSettingsStore().fetch(true)
    if ('zenxiang_liyu_enabled' in flags) await useZenxiangLiyuStore().loadStatus()
    emit('saved', settings.value)
  } catch (e) { error.value = extractApiErrorMessage(e, t('common.error')) }
  finally { saving.value = false }
}
</script>
