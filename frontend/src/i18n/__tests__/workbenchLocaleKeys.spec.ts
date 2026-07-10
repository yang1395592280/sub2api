import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const requiredKeys = [
  'nav.workbench',
  'nav.adminWorkbench',
  'nav.zenxiangLiyuOps',
  'nav.userSpendingRanking',
  'workbench.title',
  'workbench.description',
  'workbench.conversations',
  'workbench.conversationsHint',
  'workbench.newConversation',
  'workbench.retentionNotice',
  'workbench.loading',
  'workbench.untitled',
  'workbench.emptyConversation',
  'workbench.messagesCount',
  'workbench.deleteConversation',
  'workbench.emptyState',
  'workbench.workspaceHint',
  'workbench.modeChat',
  'workbench.modeImage',
  'workbench.you',
  'workbench.assistant',
  'workbench.thinking',
  'workbench.emptyMessages',
  'workbench.imagePlaceholder',
  'workbench.chatPlaceholder',
  'workbench.imageHint',
  'workbench.chatHint',
  'workbench.sending',
  'workbench.send',
  'workbench.settings',
  'workbench.settingsHint',
  'workbench.apiKey',
  'workbench.selectApiKey',
  'workbench.model',
  'workbench.selectModel',
  'workbench.endpoint',
  'workbench.imageWorkflow',
  'workbench.imageWorkflowGenerate',
  'workbench.imageWorkflowEdit',
  'workbench.referenceImage',
  'workbench.referenceImageHint',
  'workbench.removeReferenceImage',
  'workbench.inputFidelity',
  'workbench.imageSize',
  'workbench.imageQuality',
  'workbench.imageBackground',
  'workbench.imageFormat',
  'workbench.imageCompression',
  'workbench.imageCount',
  'workbench.chatPanelTitle',
  'workbench.chatPanelBody',
  'workbench.routeOnlyHint',
  'workbench.imageApiDocs',
  'workbench.referenceImageReadFailed',
  'workbench.createConversationFailed',
  'workbench.deleteConversationSuccess',
  'workbench.deleteConversationFailed',
  'workbench.apiKeyRequired',
  'workbench.modelRequired',
  'workbench.referenceImageRequired',
  'workbench.sendFailed',
  'workbench.loadFailed',
  'admin.workbench.title',
  'admin.workbench.description',
  'admin.workbench.totalConversations',
  'admin.workbench.totalMessages',
  'admin.workbench.imageMessages',
  'admin.workbench.expiredConversations',
  'admin.workbench.imageBytes',
  'admin.workbench.searchPlaceholder',
  'admin.workbench.allModes',
  'admin.workbench.allStatuses',
  'admin.workbench.hasImages',
  'admin.workbench.selectedCount',
  'admin.workbench.deleteSelected',
  'admin.workbench.cleanupExpired',
  'admin.workbench.user',
  'admin.workbench.conversation',
  'admin.workbench.mode',
  'admin.workbench.images',
  'admin.workbench.updatedAt',
  'admin.workbench.loadFailed',
  'admin.workbench.detailLoadFailed',
  'admin.workbench.deleteSelectedSuccess',
  'admin.workbench.deleteSelectedFailed',
  'admin.workbench.cleanupSuccess',
  'admin.workbench.cleanupFailed'
] as const

const additionalUiKeys = [
  'admin.accounts.channelPrice',
  'admin.accounts.channelPriceHint',
  'admin.accounts.channelPriceInvalid',
  'admin.accounts.columns.upstreamGroup',
  'admin.accounts.columns.channelPrice',
  'admin.settings.site.joinGroup.enabled',
  'admin.settings.site.joinGroup.enabledHint',
  'admin.settings.site.joinGroup.url',
  'admin.settings.site.joinGroup.urlPlaceholder',
  'admin.settings.site.joinGroup.urlHint',
  'admin.settings.site.joinGroup.image',
  'admin.settings.site.joinGroup.imagePlaceholder',
  'admin.settings.site.joinGroup.imageHint',
  'admin.dashboard.spendingRankingDescription',
  'admin.dashboard.spendingRankingUserId',
  'admin.dashboard.spendingRankingWindow',
  'admin.dashboard.rankUsers',
  'admin.dashboard.viewAllRanking',
  'admin.dashboard.sortBy',
  'admin.dashboard.sortAsc',
  'admin.dashboard.sortDesc',
  'admin.dashboard.exportRanking',
  'zenxiangLiyu.title',
  'zenxiangLiyu.description',
  'zenxiangLiyu.refresh',
  'zenxiangLiyu.currentBalance',
  'zenxiangLiyu.ticketAmount',
  'zenxiangLiyu.remainingPlays',
  'zenxiangLiyu.playHint',
  'zenxiangLiyu.open',
  'zenxiangLiyu.configuredRewards',
  'zenxiangLiyu.rewardResult',
  'zenxiangLiyu.insufficientBalance',
  'zenxiangLiyu.dailyLimitReached',
  'zenxiangLiyu.balanceUnit',
  'admin.zenxiangLiyu.title',
  'admin.zenxiangLiyu.description',
  'admin.zenxiangLiyu.settings',
  'admin.zenxiangLiyu.prizes',
  'admin.zenxiangLiyu.stats',
  'admin.zenxiangLiyu.simulator',
  'admin.zenxiangLiyu.settingsTitle',
  'admin.zenxiangLiyu.settingsHint',
  'admin.zenxiangLiyu.ticketAmount',
  'admin.zenxiangLiyu.minimumBalance',
  'admin.zenxiangLiyu.dailyPlayLimit',
  'admin.zenxiangLiyu.grants',
  'admin.zenxiangLiyu.grantSearchPlaceholder',
  'admin.zenxiangLiyu.noGrants',
  'admin.zenxiangLiyu.prizeTitle',
  'admin.zenxiangLiyu.prizeHint',
  'admin.zenxiangLiyu.savePrizeConfiguration',
  'admin.zenxiangLiyu.probabilityWarning',
  'admin.zenxiangLiyu.systemRevenue',
  'admin.zenxiangLiyu.systemExpense',
  'admin.zenxiangLiyu.systemProfit',
  'admin.zenxiangLiyu.simulatorTitle',
  'admin.zenxiangLiyu.simulatorHint',
  'admin.zenxiangLiyu.recommend',
  'admin.zenxiangLiyu.runSimulation'
] as const

function valueAtPath(messages: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((node, segment) => {
    if (node && typeof node === 'object' && segment in node) {
      return (node as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

describe.each([
  ['zh', zh],
  ['en', en]
])('workbench locale keys for %s', (_locale, messages) => {
  it('defines all keys used by workbench routes and navigation', () => {
    for (const key of requiredKeys) {
      expect(valueAtPath(messages, key), key).toEqual(expect.any(String))
    }
  })

  it('defines keys used by recent admin and activity screens', () => {
    for (const key of additionalUiKeys) {
      expect(valueAtPath(messages, key), key).toEqual(expect.any(String))
    }
  })
})
