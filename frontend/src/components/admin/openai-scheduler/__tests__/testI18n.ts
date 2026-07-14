import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'

export function createSchedulerTestI18n() {
  return createI18n({ legacy: false, locale: 'zh', messages: { zh: runtimeMessages(zh) } })
}

function runtimeMessages(value: unknown): unknown {
  if (typeof value === 'string') {
    return (context: { named: (key: string) => unknown }) =>
      value.replace(/\{(\w+)\}/g, (_match, key: string) => String(context.named(key)))
  }
  if (Array.isArray(value)) return value.map(runtimeMessages)
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, child]) => [key, runtimeMessages(child)])
    )
  }
  return value
}
