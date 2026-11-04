# Community semantic rebase acceptance - 2026-11-07

## Baseline and history

- Official repository: Wei-Shaw/sub2api, `origin/main` at `6b4b37db43443b5e88e424fe3c69602cc3810dfb` (0.2.20); 74 official commits after the former baseline `876b96d204855ad1aac486b7dd39a95d377d4760`.
- Original local head: `dbefafe035bd5cc3182e9be8da303a94419aac26`. Six local feature commits are retained; each adaptation and regression test belongs to its feature commit, not a trailing adaptation commit.
- Backup: `codex/backup-before-community-rebase-20261107` and an external Git bundle. No push, tag update, production migration, deployment or service restart is part of this rebase.
- The Nov 5 report described an older baseline and five commits. It is historical evidence only; its original text remains in `8cc2ae9ea35f1911d01c88f4516dfcea44df591c:backend/OFFICIAL_REBASE_ACCEPTANCE.md`.

## Semantic adaptation by feature

1. **Shard isolation and managed keys:** carry all three shard fields through the official auth SQL projection and cache snapshot; version 28 invalidates incomplete version 27 entries using the existing mechanism. Preserve the hydrated-context fast path without adding per-request group reads. Apply existing eligibility to media-owner selection, Live follow-up connections, Gemini cache PATCH and model metadata. Check before charging, mutation leases or upstream calls; leave resource DELETE and background recovery usable after withdrawal. Pair the Ent group/shard inverse through the existing `group_id` with cascade instead of generating a nonexistent column. Let omitted create capacity use the schema default, retain positive values, reject negatives and return the persisted value to the caller.
2. **Subscription balance fallback:** preserve official synchronous pricing/usage snapshots while carrying the explicit billing source. Deferred work does not read reused Gin contexts; SystemOne retains only the copied forward result rather than its large response parent.
3. **Client bootstrap:** only an explicitly unconfigured default group becomes `needs_setup`; repository failures and cancellation retain their error chain and are not disguised as successful setup responses.
4. **Package registry and metering:** snapshot billing source/package bindings before enqueueing, preserve daily/weekly quota hints and inline persistence, enable quota application for package-only usage, and notify using applied amounts. Keep bounded notification workers and shutdown ownership. Real PostgreSQL regressions cover package-only/mixed costs, total/daily/weekly/combined/stale hints, retry idempotency, revenue and outbox writes.
5. **Shared shard accounts and administration:** retain parent-group membership validation, shared/exclusive union, transactional writers and last-user cleanup. Combine the corrected parent Ent edge with shared account relations and regenerate; do not choose a stale generated-code conflict side. Existing picker batching, complete pagination and viewport-safe scrolling remain unchanged.
6. **Theme and persistent layout:** preserve official route authentication/feature metadata, client model defaults, Codex features, Antigravity quota groups and shutdown cleanup. Deliver desktop-login tickets only after the matching navigation succeeds; invalid, cancelled, replaced or late callbacks do not destroy the previous session. Cover one-time Redis consumption/TTL and server-side user/credential validation.

## Verification

Backend acceptance uses native Ubuntu WSL, Go 1.27.3, `GOMAXPROCS=4` and `go test -p 2`; full suites use `CI=1`, `-count=1` and `-timeout=15m`. The table records acceptance before the test-helper coverage correction described below. The delivered tree differs from that frozen full-source snapshot only in the corrected helper and this report; production source remains identical. The corrected backend snapshot is compared separately, permitting only this report to differ. Frontend content is unchanged from the tested tree `79ff541ad72866ebe5361298630a9fcd7821bc16`.

| Check | Result | Evidence |
| --- | --- | --- |
| Full backend `go test -p 2 ./... -count=1 -timeout=15m` | PASS, 213.666 seconds elapsed | `final-all-allocation-default.log` |
| Full backend `go test -p 2 -tags=unit ./... -count=1 -timeout=15m` | PASS, 384.995 seconds elapsed | `final-all-allocation-unit.log` |
| Full backend `go test -p 2 -tags=integration ./... -count=1 -timeout=15m` | PASS, 296.768 seconds elapsed | `final-all-allocation-integration.log`; real PostgreSQL/Redis testcontainers and SQL migrations |
| Six process-isolated allocation tests, each `-count=10` | 60 runs PASS; original bodies, counts and assertions unchanged | `final-all-allocation-isolated.log` |
| Inflight memory regression, `-count=10` | 10 runs PASS; 20,000 random models and original 8 MiB ceiling | `final-memory.log` |
| High-risk service/handler/repository tests with `-race -tags=unit` | PASS, no race reported in the selected cases | `final-race.log`; not a full race run |
| Native `go generate ./ent` and `go generate ./cmd/server` | PASS, zero generated-source diff | `final-ent.log`, `final-wire.log` |
| Production `CGO_ENABLED=0 go build -p 2 -tags embed -trimpath ... ./cmd/server` and embedded web tests | PASS; binary compiled, not started | `final-build.log`, `final-embed-tests.log` |
| Frontend `pnpm test:run` | 402 files / 3302 tests PASS | `frontend-full-final.log` |
| Frontend `pnpm build` | PASS, including locale checks, vue-tsc and Vite | `frontend-build-final.log` |
| Frontend `ESLINT_USE_FLAT_CONFIG=false pnpm lint:check` | PASS with this repository's legacy ESLint configuration | `frontend-lint-final.log` |

Raw logs and manifests are external to the repository, under the task's `community-rebase-20261107` audit directory and `/tmp/sub2api-semantic-rebase-20261107`. The original failed runs remain recorded; later passing reruns do not overwrite them. All seven original local SQL migration file names and blobs are unchanged. Ent/Wire/build evidence applies to identical production source; the final allocation harness changes test files only.

## Architecture rereview

- Reviewed the semantic adaptation against both the original six local commits and the official baseline, not just whether conflicts disappeared. The inspected production paths revealed no additional confirmed defect requiring a rewrite. Existing hydrated-group/cache, scope resolution, billing transaction, bounded notification worker and router navigation ownership remain the integration points; no alternate scheduler, duplicate billing path or production compatibility layer was added.
- The rereview did find an introduced test-harness defect: subprocesses executed their assertions but did not contribute counters to the parent coverage report. The helper now forwards only a nonempty `test.gocoverdir`, allowing Go to merge child counters without sharing a `coverprofile` output filename. The correction belongs to the shard commit that introduced the helper.
- Under `-covermode=count`, the model-mapping test's target counters now exactly match direct execution (including 1001 signature calls); previously the isolated report contained zero target counters. An external falsification check confirms that the body runs the warmup plus all five measurements, and an intentionally failing child assertion causes the parent to exit nonzero with the original error.
- The corrected helper is checked with all six allocation tests repeated ten times normally and three times with coverage. Frontend router/layout/API-exchange rereview passes 33 tests in three files; backend service/handler/repository focused regressions also pass. Full-suite results for this final test-only correction are recorded separately below rather than attributing the earlier run to changed test source.
- This last rerun uses a backend-only snapshot, so the existing cross-source catalog test skips when its frontend JSON is absent. That unchanged test is separately executed with the original full-source snapshot and passes without skipping (`architecture-catalog-check.log`); the catalog source, test and frontend JSON are identical to delivery. No fixture is added to the running snapshot and no skip condition is changed.

Final corrected-helper rerun (2026-11-08, same Go/environment/parallelism as above, offline cached dependencies with `-mod=readonly`):

| Backend full-suite mode | Result | Evidence |
| --- | --- | --- |
| Default `./... -count=1 -timeout=15m` | PASS, exit 0, 282.229 seconds | `architecture-post-coverage-default.log` |
| Unit `-tags=unit ./... -count=1 -timeout=15m` | PASS, exit 0, 355.672 seconds | `architecture-post-coverage-unit.log` |
| Integration `-tags=integration ./... -count=1 -timeout=15m` | PASS, exit 0, 223.604 seconds | `architecture-post-coverage-integration.log`; independent PostgreSQL/Redis testcontainers and real migrations |

All 3586 backend snapshot files remain unchanged throughout these runs. The source manifest, exact commands, exits and durations are in `architecture-post-coverage-source-manifest.json` and `architecture-post-coverage-results.json`. The six allocation cases additionally pass 60 normal and 18 coverage repetitions; counter equality and intentional-failure propagation are recorded in `allocation-coverage-fix/results.json`.

## Failures reproduced and corrected

- The first full integration run exposed a local regression: group creation explicitly wrote capacity 0 over Ent's default 2. The 0/7/-1 regression now passes, including updating the same caller object. The matching official group subset passed; this was not blamed on upstream or Docker.
- A fresh database using the real SQL migrations exposed an invalid Ent eager-loading edge that expected a second `group_shards` column. Correct inverse pairing removes that virtual column; real parent/shard traversals and generated output checks pass. Reproducible generation alone did not establish relationship correctness.
- Historical memory-fixture correction, already present in the original local shard commit `dc6ffa061`: use `NewGatewayService` rather than bypassing resolver initialization, and keep the service alive through GC. This rebase preserves that correction; it is not newly introduced work. The original local head and delivered `billing_inflight_reservation_test.go` have identical blobs. The 20,000 random model workload and 8 MiB ceiling remain unchanged.
- The initial final full unit/integration suites failed the unchanged orphan-precheck allocation assertion (51/48 allocations); a first isolated correction then exposed the same measurement problem in the thinking-filter assertion (48, limit below 16). Profiling a full local run reproduced 23 allocations/run with no GC, thousands of content-moderation workers, and context/timer/runtime-snapshot allocation stacks. A controlled interference experiment measured 0, 48, 51, then 0 with the same precheck. `testing.AllocsPerRun` counts process-wide `MemStats.Mallocs`, not only the measured function. A shared test-only helper now executes all six allocation tests in separate instances of the same binary, propagates failures and limits each child to 30 seconds. All inputs, iterations, thresholds and assertions are unchanged. The official non-race isolated test and official full unit service suite passed; the full local failure is not claimed to have been reproduced officially. No production worker-lifecycle redesign was introduced to fix a test measurement boundary.
- An additional orphan-only allocation check under `-race -count=3` failed once with actual 1, without a data-race report. The unchanged official test at `6b4b37db` also failed with actual 1: once in a same-process group of 10, and twice in a fresh-process group of 10. The exact source of that extra race-mode allocation is not established, and matching symptoms do not prove identical causality. This is a preserved acceptance limitation, not a passing result, and no assertion or test was disabled to conceal it.
- Scope and desktop-login behavior had regression tests fail before their corresponding fixes. Early toolchain or fixture compile errors were not counted as behavioral Red results.

## Scope and limitations

- Shared/exclusive visibility is preserved, but strict exclusive-first/shared-fallback ordering was already absent and is not introduced by this rebase. No provider auto-allocation, new scheduling tier, rank, sticky or WaitPlan policy is claimed.
- Automated router tests are not browser/desktop E2E acceptance. No real provider request or upgrade against a copy of an existing deployment database was exercised; database tests use independent PostgreSQL/Redis testcontainers and real migrations.
- Existing test skips remain unchanged; Docker is mandatory in the full integration run via `CI=1`. No tests were disabled to conceal a failure. Targeted race coverage is not a full race run; no performance benchmark is claimed.
- Process isolation removes the measured suite-worker interference; it does not guarantee zero fluctuation under race instrumentation. The child timeout is 30 seconds and does not inherit a shorter parent deadline. Existing content-moderation worker lifetime behavior was diagnosed but remains unchanged.
- `golangci-lint` is unavailable in this environment and was not run. Frontend build retains bundle-size and stale Browserslist warnings.
- Ignored `codebase-analysis/` documents were used only for navigation, not as proof; their old version/line references were not comprehensively regenerated. The frontend router README was updated with the actual layout/login flow.
