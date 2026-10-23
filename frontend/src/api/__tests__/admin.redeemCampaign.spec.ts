import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createCampaign, updateCampaign, listCampaigns, getCampaign, generate } from '../admin/redeem'

const { get, post, put } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get, post, put } }))

describe('redeem campaign API contract', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: [] })
    put.mockResolvedValue({ data: {} })
  })

  it.each(['balance', 'lottery_ticket'] as const)('sends the campaign ID when generating %s codes', async (type) => {
    await generate(20, type, 1, undefined, undefined, undefined, 7)
    expect(post).toHaveBeenCalledWith('/admin/redeem-codes/generate', { count: 20, type, value: 1, campaign_id: 7 })
  })

  it('keeps ordinary code generation independent of campaigns', async () => {
    await generate(1, 'balance', 10)
    expect(post).toHaveBeenCalledWith('/admin/redeem-codes/generate', { count: 1, type: 'balance', value: 10 })
  })

  it('passes the operation key and editable limit on creation and update', async () => {
    const input = { name: '福利', per_user_limit: 3 }
    await createCampaign(input, 'create-operation')
    expect(post).toHaveBeenCalledWith('/admin/redeem-codes/campaigns', input, { headers: { 'Idempotency-Key': 'create-operation' } })
    await updateCampaign(7, { ...input, per_user_limit: 5 }, 'edit-operation')
    expect(put).toHaveBeenCalledWith('/admin/redeem-codes/campaigns/7', { name: '福利', per_user_limit: 5 }, { headers: { 'Idempotency-Key': 'edit-operation' } })
  })

  it('paginates campaign search and loads older campaigns by ID', async () => {
    await listCampaigns(2, '周五')
    expect(get).toHaveBeenCalledWith('/admin/redeem-codes/campaigns', { params: { page: 2, page_size: 20, search: '周五' } })
    await getCampaign(7)
    expect(get).toHaveBeenLastCalledWith('/admin/redeem-codes/campaigns/7')
  })
})
