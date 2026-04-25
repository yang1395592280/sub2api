import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

const storageStub = {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}

vi.stubGlobal('localStorage', storageStub)

vi.mock('../SizeBetGameView.vue', () => ({
  default: {
    props: ['embedded'],
    template: '<div data-test="size-bet-embedded">size-bet-embedded-{{ embedded }}</div>',
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        const dict: Record<string, string> = {
          'gameCenter.shell.back': '返回游戏中心',
          'gameCenter.shell.openRaw': '新标签打开',
          'gameCenter.shell.titlePrefix': '全屏模式',
          'gameCenter.shell.unsupportedTitle': '不支持的游戏',
          'gameCenter.shell.unsupportedDescription': '暂不支持打开',
        }
        return dict[key] ?? key
      },
    }),
  }
})

async function mountView(path = '/game-center/size_bet') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/game-center/:gameKey', component: { template: '<div />' } },
      { path: '/game-center', component: { template: '<div />' } },
    ],
  })
  await router.push(path)
  await router.isReady()

  const { default: GameCenterShellView } = await import('../GameCenterShellView.vue')
  return mount(GameCenterShellView, {
    global: {
      plugins: [router],
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        EmptyState: { template: '<div data-test="empty-state"><slot /></div>' },
      },
    },
  })
}

describe('GameCenterShellView', () => {
  it('renders the actual size bet component instead of an iframe', async () => {
    const wrapper = await mountView('/game-center/size_bet')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('[data-test="size-bet-embedded"]').text()).toContain('true')
  })
})
