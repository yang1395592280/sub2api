<template>
  <div v-if="showJoinGroup">
    <a
      v-if="!hasPopupImage"
      :href="joinGroupUrl"
      target="_blank"
      rel="noopener noreferrer"
      :class="linkClass"
      :title="title"
    >
      <slot />
    </a>

    <button
      v-else
      type="button"
      :class="linkClass"
      :title="title"
      @click="dialogOpen = true"
    >
      <slot />
    </button>

    <BaseDialog
      :show="dialogOpen"
      :title="dialogTitle"
      width="narrow"
      :close-on-click-outside="true"
      @close="dialogOpen = false"
    >
      <div class="space-y-4">
        <img
          :src="joinGroupPopupImage"
          alt="Join group"
          class="max-h-[70vh] w-full rounded-xl object-contain"
        />
        <a
          v-if="joinGroupUrl"
          :href="joinGroupUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="btn btn-primary w-full justify-center"
        >
          {{ actionText }}
        </a>
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '@/stores'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  linkClass: string
  title: string
  dialogTitle?: string
  actionText?: string
}>()

const appStore = useAppStore()
const dialogOpen = ref(false)

const joinGroupEnabled = computed(
  () => appStore.cachedPublicSettings?.join_group_enabled === true,
)
const joinGroupUrl = computed(() =>
  String(appStore.cachedPublicSettings?.join_group_url || '').trim(),
)
const joinGroupPopupImage = computed(() =>
  String(appStore.cachedPublicSettings?.join_group_popup_image || '').trim(),
)
const hasPopupImage = computed(() => joinGroupPopupImage.value !== '')
const showJoinGroup = computed(
  () =>
    joinGroupEnabled.value &&
    (joinGroupUrl.value !== '' || joinGroupPopupImage.value !== ''),
)

const dialogTitle = computed(() => props.dialogTitle || '加入群聊')
const actionText = computed(() => props.actionText || '立即跳转')
</script>
