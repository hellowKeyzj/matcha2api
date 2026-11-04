# Group Shard Account Isolation

A shard restricts account eligibility inside its parent group. It does not
introduce a scheduling tier, account preference, or fallback policy.

- Shared accounts belong to the parent group's shared pool and are eligible
  for every active shard in that group.
- Exclusive accounts belong to exactly one shard in the parent group.
- An account cannot be both shared and exclusive within the same parent group.
- Accounts must belong to the parent group before being configured in either pool.
- A user's eligible set is their shard's exclusive accounts plus the shared pool,
  filtered by the requested provider. Existing scheduling rules apply unchanged.
- Sticky-session eligibility uses the same set as ordinary account selection.

## Lifecycle

Deleting or removing the last live user deletes the shard and its account
bindings in the same transaction. Provider accounts and parent-group account
membership are preserved. Shared accounts are unaffected.

Manually created shards can be configured before their first member is assigned.
The empty-shard cleanup runs when members are removed, not on reads.
The migration removes historical empty shards and deleted-user memberships.

## Admin API

- `GET /api/v1/admin/groups/:id/shared-accounts` lists the shared pool.
- `PUT /api/v1/admin/groups/:id/shared-accounts` replaces the shared pool with
  `{ "accounts": [{ "account_id": 123, "priority": 50 }] }`.
- Existing shard account endpoints configure exclusive accounts only.

To move an account between shared and exclusive pools, first remove its existing
binding, then add it to the destination pool. Conflicting assignments are rejected.
