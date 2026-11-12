import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import { i18n, loadLocaleMessages } from '@/i18n'

describe('runtime locale messages', () => {
  it('translates the actual lazy-loaded English and Chinese messages', async () => {
    await loadLocaleMessages('en')
    await loadLocaleMessages('zh')

    expect(i18n.global.t('common.save', {}, { locale: 'en' })).toBe('Save')
    expect(i18n.global.t('common.save', {}, { locale: 'zh' })).toBe('保存')
  })

  it('supports interpolation and plural messages under the application build settings', () => {
    const instance = createI18n({
      legacy: false,
      locale: 'en',
      messages: { en: { greeting: 'Hello, {name}', count: 'no items | one item | {count} items' } }
    })

    const greetingKey = 'greeting'
    const countKey = 'count'
    expect(instance.global.t(greetingKey, { name: 'Ada' })).toBe('Hello, Ada')
    expect(instance.global.t(countKey, 0)).toBe('no items')
    expect(instance.global.t(countKey, 1)).toBe('one item')
    expect(instance.global.t(countKey, 3)).toBe('3 items')
  })
})
