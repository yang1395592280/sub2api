import { beforeEach, describe, expect, it, vi } from 'vitest'
const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { post } }))
import { updateBalance } from '../admin/users'
beforeEach(() => { vi.clearAllMocks(); post.mockResolvedValue({ data: { id: 7 } }) })
describe('admin wallet request contract', () => {
  it.each(['checkin', 'liyu', 'gift'] as const)('sends the selected %s source with a stable refund key', async source => {
    await updateBalance(7, 0, 'subtract', 'refund', 1.12345678, source, 'stable-key')
    expect(post).toHaveBeenCalledWith('/admin/users/7/balance', { balance: 0, operation: 'subtract', notes: 'refund', welfare_balance: 1.12345678, welfare_source: source }, { headers: { 'Idempotency-Key': 'stable-key' } })
  })
  it('keeps pure paid refunds compatible and idempotent', async () => {
    await updateBalance(7, 2, 'subtract')
    expect(post).toHaveBeenCalledWith('/admin/users/7/balance', { balance: 2, operation: 'subtract', notes: '' }, { headers: { 'Idempotency-Key': expect.any(String) } })
  })
  it('preserves the existing welfare gift request', async () => {
    await updateBalance(7, 2, 'add', '', 3)
    expect(post).toHaveBeenCalledWith('/admin/users/7/balance', { balance: 2, operation: 'add', notes: '', welfare_balance: 3 }, { headers: { 'Idempotency-Key': expect.any(String) } })
  })
})
