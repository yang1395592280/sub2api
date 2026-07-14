import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('openai auto scheduler locale copy', () => {
  it('defines the scheduler console contract in Chinese and English', () => {
    expect(zh.admin.openaiAutoScheduler.title).toBe('OpenAI 调度控制台')
    expect(en.admin.openaiAutoScheduler.title).toBe('OpenAI Scheduler Console')

    const keys = [
      'tabs.overview',
      'tabs.health',
      'tabs.events',
      'tabs.settings',
      'overview.e2eP50',
      'overview.e2eP90',
      'overview.selectionP95',
      'overview.probeRatio',
      'health.realSamples',
      'health.probeSamples',
      'health.decision.contextRequired',
      'events.scoreChange',
      'settings.shadowMode',
      'settings.sessionEscapeGap',
      'actions.manualProbe',
      'actions.resetHealth',
    ]

    for (const key of keys) {
      expect(resolve(zh.admin.openaiAutoScheduler, key)).toBeTruthy()
      expect(resolve(en.admin.openaiAutoScheduler, key)).toBeTruthy()
    }
  })
})

function resolve(root: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((value, segment) => {
    if (!value || typeof value !== 'object') return undefined
    return (value as Record<string, unknown>)[segment]
  }, root)
}
