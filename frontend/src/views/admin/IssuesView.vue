<template>
  <section class="mx-auto max-w-[1200px] space-y-5" :aria-label="t('nav.desktopIssues')">
    <template v-if="showList">
      <h1 class="text-xl font-semibold tracking-tight text-gray-900 dark:text-white lg:hidden">
        {{ t('nav.desktopIssues') }}
      </h1>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex min-w-0 flex-1 items-center gap-2 sm:flex-none">
          <div class="relative min-w-0 flex-1 sm:w-64">
            <Icon name="search" size="sm" aria-hidden="true" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-500 dark:text-dark-400" />
            <input
              v-model="filters.search"
              type="search"
              class="input h-10 pl-9 text-sm"
              :placeholder="t('admin.issues.searchPlaceholder')"
              :aria-label="t('common.search')"
              @input="resetPageAndLoad(240)"
            />
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-icon h-10 w-10 shrink-0"
            :disabled="listLoading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="loadList()"
          >
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': listLoading }" aria-hidden="true" />
          </button>
        </div>

        <div class="flex w-full flex-wrap items-center gap-2 sm:w-auto">
          <Select
            v-model="filters.kind"
            class="min-w-0 flex-1 sm:min-w-32 sm:flex-none"
            :options="kindOptions"
            :aria-label="t('admin.issues.kind.all')"
            @change="resetPageAndLoad()"
          />
          <Select
            v-model="filters.severity"
            class="min-w-0 flex-1 sm:min-w-32 sm:flex-none"
            :options="severityOptions"
            :aria-label="t('admin.issues.severity.all')"
            @change="resetPageAndLoad()"
          />
          <button
            type="button"
            class="btn btn-secondary h-10 shrink-0 whitespace-nowrap text-xs"
            :class="{ 'font-semibold ring-1 ring-primary-500/60': filters.hasDiagnosticsOnly }"
            :aria-pressed="filters.hasDiagnosticsOnly"
            @click="toggleDiagnosticsFilter"
          >
            {{ t('admin.issues.filters.diagnosticsOnly') }}
          </button>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <div class="inline-flex flex-wrap items-center gap-1 rounded-xl border border-gray-200 bg-surface p-1 dark:border-dark-700 dark:bg-dark-900" role="group" :aria-label="t('common.status')">
          <button
            v-for="status in statuses"
            :key="status"
            type="button"
            class="inline-flex min-h-8 items-center gap-1 rounded-lg px-1.5 text-xs font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 sm:px-2.5"
            :aria-pressed="filters.status === status"
            :class="filters.status === status
              ? 'bg-gray-100 text-gray-900 dark:bg-dark-800 dark:text-white'
              : 'text-gray-600 hover:text-gray-900 dark:text-dark-400 dark:hover:text-gray-200'"
            @click="setStatusFilter(status)"
          >
            <span v-if="status !== 'all'" class="hidden h-1.5 w-1.5 rounded-full sm:block" :class="statusStyle[status].dot" aria-hidden="true" />
            {{ t(`admin.issues.status.${status}`) }}
            <span class="font-mono text-[11px] opacity-70">{{ list?.counts[status] ?? 0 }}</span>
          </button>
        </div>
        <button
          v-if="filters.reporter"
          type="button"
          class="inline-flex max-w-full items-center gap-1 rounded-full border border-primary-500/30 bg-primary-500/10 px-2.5 py-1 text-xs text-primary-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-primary-400"
          :aria-label="t('admin.issues.actions.clearReporter', { email: filters.reporter })"
          @click="filterByReporter('')"
        >
          <span class="truncate">{{ t('admin.issues.reporterFilter', { email: filters.reporter }) }}</span>
          <Icon name="x" size="xs" class="shrink-0" aria-hidden="true" />
        </button>
      </div>

      <div class="overflow-hidden rounded-2xl border border-gray-200 bg-surface shadow-card dark:border-dark-700 dark:bg-dark-900" :aria-busy="listLoading">
        <div class="flex min-h-11 flex-wrap items-center gap-x-4 gap-y-2 border-b border-gray-200 bg-gray-50/80 px-3 py-2 text-xs text-gray-600 dark:border-dark-700 dark:bg-dark-800/70 dark:text-dark-400 sm:px-4">
          <label class="flex items-center gap-2.5">
            <input
              type="checkbox"
              class="h-4 w-4 shrink-0 rounded accent-primary-500"
              :aria-label="t('common.selectAll')"
              :checked="items.length > 0 && selectedIds.size === items.length"
              :indeterminate="selectedIds.size > 0 && selectedIds.size < items.length"
              :disabled="listLoading || !!listError || bulkBusy || items.length === 0"
              @change="toggleSelectAll(($event.target as HTMLInputElement).checked)"
            />
            <span aria-live="polite">{{ selectedIds.size > 0
              ? t('admin.issues.selectedCount', { count: selectedIds.size })
              : t('admin.issues.totalCount', { count: list?.total ?? 0 }) }}</span>
          </label>
          <div v-if="selectedIds.size > 0" class="flex w-full flex-wrap gap-2 sm:w-auto" role="group" :aria-label="t('common.actions')">
            <button
              v-for="status in bulkStatuses"
              :key="status"
              type="button"
              class="btn btn-secondary btn-sm min-h-8"
              :disabled="bulkBusy"
              @click="runBulkStatus(status)"
            >
              {{ t(`admin.issues.actions.bulk.${status}`) }}
            </button>
          </div>
        </div>

        <div v-if="listLoading" class="divide-y divide-gray-100 dark:divide-dark-800" :aria-label="t('common.loading')">
          <div v-for="i in 4" :key="i" class="space-y-2 px-4 py-4" aria-hidden="true">
            <div class="h-4 w-2/5 animate-pulse rounded bg-gray-200 dark:bg-dark-700" />
            <div class="h-3 w-1/3 animate-pulse rounded bg-gray-100 dark:bg-dark-800" />
          </div>
        </div>
        <p v-else-if="listError" role="alert" class="px-4 py-12 text-center text-sm text-gray-700 dark:text-gray-200">
          {{ listError }}
        </p>
        <p v-else-if="items.length === 0" class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
          {{ t('admin.issues.empty') }}
        </p>

        <ul v-else class="divide-y divide-gray-100 dark:divide-dark-800">
          <li
            v-for="issue in items"
            :key="issue.id"
            class="group flex items-start gap-2 px-3 py-2.5 hover:bg-gray-50 dark:hover:bg-dark-800/50 sm:gap-3 sm:px-4"
          >
            <label class="grid h-8 shrink-0 cursor-pointer place-items-center">
              <input
                type="checkbox"
                class="h-4 w-4 rounded accent-primary-500"
                :aria-label="t('admin.issues.selectIssue', { id: issue.id })"
                :checked="selectedIds.has(issue.id)"
                :disabled="bulkBusy"
                @change="toggleRowSelection(issue.id, ($event.target as HTMLInputElement).checked)"
              />
            </label>
            <RouterLink
              :to="{ name: 'AdminIssueDetail', params: { id: issue.id } }"
              class="grid min-w-0 flex-1 gap-x-4 gap-y-2 rounded-md p-1 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 md:grid-cols-[minmax(0,1fr)_auto]"
            >
              <div class="min-w-0 space-y-1.5">
                <p class="break-words text-sm font-semibold text-gray-900 group-hover:text-primary-600 dark:text-gray-100 dark:group-hover:text-primary-400">
                  {{ issue.title }}
                </p>
                <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                  <span class="font-mono">#{{ issue.id }}</span>
                  <span class="inline-flex items-center gap-1 rounded-full border px-1.5 py-0.5 text-[11px] font-medium" :class="statusStyle[issue.status].badge">
                    <span class="h-1.5 w-1.5 rounded-full" :class="statusStyle[issue.status].dot" aria-hidden="true" />
                    {{ t(`admin.issues.status.${issue.status}`) }}
                  </span>
                  <span class="max-w-full truncate">{{ reporterDisplay(issue) }}</span>
                  <span v-if="issue.app_version" class="max-w-full truncate font-mono">{{ issue.app_version }}</span>
                  <span v-if="issue.client_os" class="max-w-full truncate">{{ shortOs(issue.client_os) }}</span>
                  <span>{{ formatRelativeTime(issue.created_at) }}</span>
                </div>
                <div v-if="issue.kind !== 'feature' || issue.labels.length || issue.fix_version" class="flex flex-wrap gap-1 text-[11px]">
                  <span v-if="issue.kind !== 'feature'" class="rounded-full px-2 py-0.5 font-medium" :class="severityStyle[issue.severity]">
                    {{ t(`admin.issues.severity.${issue.severity}`) }}
                  </span>
                  <span v-for="label in issue.labels" :key="label" class="max-w-full truncate rounded-full bg-gray-100 px-2 py-0.5 text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                    {{ label }}
                  </span>
                  <span v-if="issue.fix_version" class="max-w-full truncate rounded-full bg-primary-500/10 px-2 py-0.5 font-mono text-primary-700 dark:text-primary-400">
                    {{ issue.fix_version }}
                  </span>
                </div>
              </div>
              <div v-if="issue.has_diagnostics || issue.image_count || issue.video_count || issue.comment_count || issue.assignee_name" class="flex flex-wrap items-center gap-2 self-start text-xs tabular-nums text-gray-500 dark:text-dark-400">
                <span v-if="issue.has_diagnostics" class="rounded-full border border-primary-500/30 bg-primary-500/10 px-2 py-0.5 text-[11px] font-medium text-primary-700 dark:text-primary-400">
                  ZIP
                </span>
                <span v-if="issue.image_count > 0" class="inline-flex items-center gap-1" :title="t('admin.issues.evidence.images', { count: issue.image_count })">
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                    <rect x="3" y="3" width="18" height="18" rx="2" /><circle cx="8.5" cy="8.5" r="1.5" /><path d="m3 17 5-5 4 4 4-6 5 6" />
                  </svg>
                  {{ issue.image_count }}
                </span>
                <span v-if="issue.video_count > 0" class="inline-flex items-center gap-1" :title="t('admin.issues.evidence.videos', { count: issue.video_count })">
                  <Icon name="play" size="sm" aria-hidden="true" />{{ issue.video_count }}
                </span>
                <span v-if="issue.comment_count > 0" class="inline-flex items-center gap-1" :title="t('admin.issues.evidence.notes', { count: issue.comment_count })">
                  <Icon name="chatBubble" size="sm" aria-hidden="true" />{{ issue.comment_count }}
                </span>
                <span v-if="issue.assignee_name" class="grid h-6 w-6 place-items-center rounded-full bg-primary-500/10 font-semibold text-primary-700 dark:text-primary-400" :title="issue.assignee_name">
                  {{ initialChar(issue.assignee_name) }}
                </span>
              </div>
            </RouterLink>
          </li>
        </ul>
        <Pagination
          v-if="!listLoading && !listError && list && list.pages > 1"
          :total="list.total"
          :page="list.page"
          :page-size="list.page_size"
          :show-page-size-selector="false"
          @update:page="goPage"
        />
      </div>
    </template>

    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-2">
        <RouterLink :to="{ name: 'AdminIssues' }" class="btn btn-secondary btn-sm gap-1.5">
          <Icon name="chevronLeft" size="sm" aria-hidden="true" />{{ t('admin.issues.allIssues') }}
        </RouterLink>
        <div class="flex items-center gap-2">
          <button
            v-for="direction in adjacentDirections"
            :key="direction"
            type="button"
            class="btn btn-secondary btn-icon h-9 w-9"
            :title="t(`admin.issues.actions.${direction}`)"
            :aria-label="t(`admin.issues.actions.${direction}`)"
            :disabled="adjacentIds[direction] == null"
            @click="openAdjacent(direction)"
          >
            <Icon :name="direction === 'prev' ? 'chevronLeft' : 'chevronRight'" size="sm" aria-hidden="true" />
          </button>
        </div>
      </div>
      <RouterView v-slot="{ Component }">
        <component :is="Component" :key="route.params.id" @filter-by-reporter="filterByReporter" />
      </RouterView>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { issuesAPI } from '@/api/admin'
import type { DesktopIssueListQuery, DesktopIssueListResponse, DesktopIssueStatus } from '@/api/admin/issues'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatRelativeTime } from '@/utils/format'
import { initialChar, issueSeverities, issueStatuses, reporterDisplay, severityStyle, shortOs, statusStyle } from './desktop-issues/presentation'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const showList = computed(() => route.name === 'AdminIssues')

const statuses = ['all', ...issueStatuses] as const
const bulkStatuses = ['in_progress', 'resolved', 'closed'] as const
const adjacentDirections = ['prev', 'next'] as const
const kindOptions = computed(() => ['all', 'bug', 'feature', 'experience'].map(value => ({ value, label: t(`admin.issues.kind.${value}`) })))
const severityOptions = computed(() => ['all', ...issueSeverities].map(value => ({ value, label: t(`admin.issues.severity.${value}`) })))

const page = ref(1)
const pageSize = 20
const list = shallowRef<DesktopIssueListResponse>()
const items = computed(() => list.value?.items ?? [])
const listLoading = ref(false)
const listError = ref('')
const bulkBusy = ref(false)
const selectedIds = reactive(new Set<number>())
const filters = reactive({ status: 'all', kind: 'all', severity: 'all', hasDiagnosticsOnly: false, reporter: '', search: '' })

const adjacentIds = computed(() => {
  const index = items.value.findIndex(issue => issue.id === Number(route.params.id))
  return index < 0 ? {} : { prev: items.value[index - 1]?.id, next: items.value[index + 1]?.id }
})

let listRequest: AbortController | undefined
let bulkRequest: AbortController | undefined
let searchTimer: ReturnType<typeof setTimeout> | undefined

function cancelList() {
  listRequest?.abort()
  clearTimeout(searchTimer)
}

function loadList(delay = 0) {
  cancelList()
  listRequest = new AbortController()
  const { signal } = listRequest
  listLoading.value = true
  listError.value = ''
  selectedIds.clear()

  const query: DesktopIssueListQuery = {
    page: page.value,
    page_size: pageSize,
    status: filters.status === 'all' ? undefined : filters.status,
    kind: filters.kind === 'all' ? undefined : filters.kind,
    severity: filters.severity === 'all' ? undefined : filters.severity,
    has_diagnostics: filters.hasDiagnosticsOnly || undefined,
    reporter: filters.reporter || undefined,
    search: filters.search.trim() || undefined
  }

  if (delay) searchTimer = setTimeout(() => void fetchList(query, signal), delay)
  else void fetchList(query, signal)
}

async function fetchList(query: DesktopIssueListQuery, signal: AbortSignal) {
  try {
    const result = await issuesAPI.list(query, { signal })
    if (signal.aborted) return
    // Filtering or a batch update can remove the final page.
    const lastPage = Math.max(1, result.pages)
    if (result.page > lastPage) {
      goPage(lastPage)
      return
    }
    list.value = result
  } catch (error) {
    if (!signal.aborted) listError.value = extractApiErrorMessage(error, t('admin.issues.listLoadFailed'))
  } finally {
    if (!signal.aborted) listLoading.value = false
  }
}

function resetPageAndLoad(delay = 0) {
  page.value = 1
  loadList(delay)
}

function setStatusFilter(status: typeof statuses[number]) {
  if (filters.status === status) return
  filters.status = status
  resetPageAndLoad()
}

function toggleDiagnosticsFilter() {
  filters.hasDiagnosticsOnly = !filters.hasDiagnosticsOnly
  resetPageAndLoad()
}

function filterByReporter(email: string) {
  filters.reporter = email
  page.value = 1
  if (showList.value) loadList()
  else void router.push({ name: 'AdminIssues' })
}

function goPage(next: number) {
  page.value = next
  loadList()
}

function toggleRowSelection(id: number, checked: boolean) {
  if (checked) selectedIds.add(id)
  else selectedIds.delete(id)
}

function toggleSelectAll(checked: boolean) {
  selectedIds.clear()
  if (checked) for (const issue of items.value) selectedIds.add(issue.id)
}

async function runBulkStatus(status: DesktopIssueStatus) {
  if (bulkBusy.value || selectedIds.size === 0) return
  bulkRequest = new AbortController()
  const { signal } = bulkRequest
  bulkBusy.value = true
  try {
    await issuesAPI.batchUpdateStatus([...selectedIds], status, { signal })
    if (!signal.aborted) loadList()
  } catch (error) {
    if (!signal.aborted) appStore.showError(extractApiErrorMessage(error, t('admin.issues.detail.updateFailed')))
  } finally {
    if (!signal.aborted) bulkBusy.value = false
  }
}

function openAdjacent(direction: typeof adjacentDirections[number]) {
  const id = adjacentIds.value[direction]
  if (id != null) void router.push({ name: 'AdminIssueDetail', params: { id } })
}

watch(showList, (visible, _, onCleanup) => {
  if (!visible) return
  bulkBusy.value = false
  loadList()
  onCleanup(() => {
    cancelList()
    bulkRequest?.abort()
  })
}, { immediate: true, flush: 'sync' })
</script>
