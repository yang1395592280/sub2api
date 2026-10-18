export const CUSTOM_FEATURE_KEYS = [
  'zenxiang_liyu_enabled', 'redeem_code_purchase_enabled', 'join_group_enabled'
] as const

export interface CustomFeatures {
  flags: Record<string, boolean>
  menus: { id: string; label: string; enabled: boolean }[]
}
