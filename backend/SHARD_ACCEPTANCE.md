# Shard Acceptance - 2026-11-04

## Automated checks

- Frontend: 28 tests pass across Select, Select keyboard, Shard view and shared API suites.
- Locale completeness: 3 tests pass.
- vue-tsc typecheck: pass.
- Legacy-config ESLint on changed picker/view/tests: pass.
- Production build: pass; existing chunk-size and mixed-import warnings remain.
- Backend repository, service, admin handler, routes and migrations: pass.
  Windows requires C:/Program Files/Git/bin on PATH for shell-based backup tests.
- PostgreSQL/Redis integration: 6 tests pass with isolated testcontainers:
  deleted-user capacity, deletion with remaining members, shared visibility/sticky,
  exclusive conflicts/atomic replacement, concurrent writes, last-member cleanup.
- git diff --check: pass (line-ending warnings only).

## Authenticated browser checks

Tested localhost:3000/admin/groups/2/shards without saving live bindings.

- Open picker, click outside, Escape and Tab close: pass.
- ArrowDown + Enter adds a draft account, closes and restores trigger focus: pass.
- Mouse selection and exclusion of already-selected draft accounts: pass.
- Remote search with no matches displays the empty state: pass.
- Shared/current-Shard navigation retains the exclusive draft: pass.
- Ten-account draft: 382px inner height, 582px content at default viewport.
- At 390x844: document width 382px; no horizontal overflow.
  Account container: 382px inner height, 822px content, scrollTop reaches 440px.
- Reload restores original two exclusive accounts and empty shared pool.
- Captured browser error logs: none.

## Regression found and fixed

A save response for Shard A could overwrite Shard B after navigation. A regression
test first failed, then passed after scoping response application to its group and
Shard identity. The account picker now reuses common Select instead of maintaining
another dropdown implementation. Initial remote loading is opt-in, preserving
other Select callers.

## Not accepted / not covered

- Exclusive-first/shared-fallback scheduling is NOT implemented. Current scope is
  the union of both pools and retains existing scheduling. This does not satisfy
  the user's explicit exclusive-first requirement; the UI hint and GROUP_SHARDS.md
  describe current behavior, not acceptance of that requirement.
- No live user/account deletion or binding save was performed in the browser.
  Data lifecycle and persistence were tested in isolated integration tests.
- No real upstream model request, billing validation or performance benchmark.
- A clean full Ent regeneration was not verified in this Windows checkout.
- Switching different Shards does not preserve unsaved exclusive drafts.

Evidence logs and screenshot are in ../acceptance-artifacts/ locally.
