# Official community rebase acceptance

Date: 2026-11-05

## Baseline and history

- Target: Wei-Shaw/sub2api, origin/main at 876b96d204855ad1aac486b7dd39a95d377d4760.
- Original local head: 7715a8837; all five feature commits are retained.
- The fork-only VERSION update is not included.
- Original and mistaken-rebase heads remain on backup branches.
- No push, production migration, or production service restart was performed.
- Rebuilt as five self-contained feature commits; there is no trailing adaptation commit. Backend unit-tag compilation of every package and frontend type checking passed at each feature boundary. Targeted shard/default-key, billing/inflight, bootstrap/localization and package regressions passed at their corresponding boundaries.
- Explicit billing-source/inflight plumbing previously placed in the package commit is now owned by the billing commit. Constructor fixture changes belong to the shard commit; bootstrap localization belongs to the bootstrap commit.

## Semantic integration

- Keep upstream partial-response usage recording; carry local billing source and package metering through its submission closure.
- Keep upstream image-size sticky routing and spillover behavior alongside shard eligibility checks.
- Keep upstream Gemini cached-content binding exclusions and selected-account validation.
- Pass the authenticated user into Gemini cache creation account selection.
- Carry explicit billing source through Gemini cache eligibility, token reservations, fixed storage charges, quotations and usage settlement. Snapshot it before asynchronous usage submission.
- Preserve upstream uncapped fixed-charge reservations and API-key/subscription headroom checks.
- Preserve both upstream MCP grant revocation and local default gateway provisioning in dependency injection.
- Localize group DTOs in key rotation and client bootstrap using upstream Accept-Language behavior.
- Adapt upstream tagged test fixtures to the combined constructor dependencies; keep localized error-message expectations consistent with the local UI contract.
- Preserve migration filenames and checksums. Equal numeric prefixes are not migration identities: the runner uses complete filenames.

## Verified

- Frontend: 388 test files, 3146 tests passed.
- Frontend production build passed, including locale checks and Vue/TypeScript compilation.
- Backend default suites passed: service, handler (including admin, DTO and quota view), repository, server middleware/routes, migrations and server command.
- Backend unit-tag handler, repository and server contract suites passed.
- Backend unit-tag targeted service regressions passed: shard, subscription balance, billing source, package, fixed charge and inflight reservation.
- All backend packages compile with the unit tag (`go test -tags unit ./... -run '^$'`).
- Independent Docker PostgreSQL/Redis run applied all migrations and passed 19 top-level integration tests covering inflight concurrency/renewal/expiry/error handling, deleted users, empty shards, shared/exclusive visibility, atomic replacement, concurrent writers, package publication and concurrent cloud-key creation.
- Ent regenerated successfully in an isolated directory. Generated output matches the checkout ignoring line endings; Wire regeneration produces no diff.
- New regressions cover balance fallback with an exhausted but nonnil subscription and localized bootstrap without mutating service data.
- `git diff --check` passed.

## Scope and limitations

- No browser acceptance against a restarted service or real-provider request was performed for this rebase.
- Fresh-database migration execution passed; upgrade against a copy of an existing deployment database was not exercised.
- Shared/exclusive account visibility is preserved. Strict exclusive-first/shared-fallback ordering was already absent and is not introduced by this rebase.
- Frontend build still reports bundle-size and stale Browserslist warnings.
- Unit-tag full service verification is recorded separately below; do not confuse default-suite success with full tagged-suite success.

## Full unit-tag service failures

- Default parallel run: three failures. Serial rerun: two failures. Isolated rerun of all three: passed.
- `TestCommandCodeGatewayPassesThroughCatalogProtocols`: also fails in a full suite from an untouched archive of official origin/main. Serial local run passed.
- `TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort`: also fails on untouched official code, both isolated and full-suite runs.
- Memory test failure is resolved as described below; the 8 MiB limit remains unchanged.
- No assertions were weakened and no tests were skipped to hide these failures. These residual failures are not a production acceptance guarantee.

## Memory failure diagnosis and correction

- The inflight test helper constructed GatewayService with a struct literal, omitting the userGroupRateResolver that NewGatewayService initializes. EstimateInflightReservation invokes the rate resolver on every call. The nil-resolver compatibility path creates a new go-cache and janitor ticker on each call; discarded caches are reclaimed asynchronously through finalizers.
- Reproduction before correction: 100 warmup requests left 102 goroutines. The 20,000-request loop created transient cache janitors rather than a retained per-model pricing cache. Heap profile diff attributed approximately 4 MiB to runtime goroutine/thread objects and additional retained memory to go-cache janitors/timers. Heap growth varied with GC and finalizer scheduling, explaining isolated passes and full-suite failures.
- Correct the fixture to use NewGatewayService, matching production initialization and cache ownership. Keep the service alive across the post-loop GC so measuring an already-dead fixture cannot hide cache retention. No production lazy-initialization layer, extra synchronization, threshold increase or unrelated routing change is introduced.
- The unchanged memory assertion passed 20 consecutive runs after correction and passed in the final full unit-tag service run (253.406 seconds). That full run had only the pre-existing Ollama stale-callback failure; the protocol-catalog test passed. All rebuilt unit-tag handler/repository/server/migration/server-command suites passed.
