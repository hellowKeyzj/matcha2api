import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put } = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get, put } }))

import { listSharedAccounts, setSharedAccounts } from '@/api/admin/groups'

describe('admin group shared accounts API', () => {
  const saved = [{ group_shard_id: 0, group_id: 10, account_id: 201, priority: 7 }]

  beforeEach(() => {
    get.mockReset().mockResolvedValue({ data: saved })
    put.mockReset().mockResolvedValue({ data: saved })
  })

  it('loads the parent pool without a shard ID', async () => {
    expect(await listSharedAccounts(10)).toEqual(saved)
    expect(get).toHaveBeenCalledWith('/admin/groups/10/shared-accounts')
  })

  it('sends explicit accounts and priorities and returns the saved array', async () => {
    const accounts = [{ account_id: 201, priority: 7 }]
    expect(await setSharedAccounts(10, accounts)).toEqual(saved)
    expect(put).toHaveBeenCalledWith('/admin/groups/10/shared-accounts', { accounts })
  })

  it('supports clearing the pool', async () => {
    put.mockResolvedValue({ data: [] })
    expect(await setSharedAccounts(10, [])).toEqual([])
    expect(put).toHaveBeenCalledWith('/admin/groups/10/shared-accounts', { accounts: [] })
  })
})
