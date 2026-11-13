<template>
  <button
    type="button"
    data-testid="header-daily-checkin"
    class="inline-flex shrink-0 items-center gap-1.5 rounded-xl border border-emerald-200 bg-emerald-50 px-2.5 py-1.5 text-sm font-medium text-emerald-700 transition hover:border-emerald-300 hover:bg-emerald-100 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300"
    :title="t('wallet.checkinTitle')"
    :aria-label="t('wallet.checkinTitle')"
    aria-haspopup="dialog"
    @click="show = true"
  >
    <Icon name="calendar" size="sm" />
    <span class="hidden sm:inline">{{ t('wallet.checkinTitle') }}</span>
  </button>
  <BaseDialog :show="show" :title="t('wallet.checkinTitle')" width="wide" @close="show = false">
    <ProfileCheckinCard v-if="show" />
  </BaseDialog>
</template>

<script setup lang="ts">
import { defineAsyncComponent, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const ProfileCheckinCard = defineAsyncComponent(() => import('@/components/user/profile/ProfileCheckinCard.vue'))
const { t } = useI18n()
const show = ref(false)
</script>
