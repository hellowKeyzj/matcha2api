<template>
  <div v-if="loadError" class="space-y-3 rounded-2xl border border-gray-200 bg-surface p-5 dark:border-dark-700 dark:bg-dark-900">
    <p role="alert" class="text-sm text-gray-700 dark:text-gray-200">{{ loadError }}</p>
    <button type="button" class="btn btn-secondary btn-sm" @click="loadDetail">{{ t('common.refresh') }}</button>
  </div>
  <div v-else-if="!detail" class="space-y-4" :aria-label="t('common.loading')" aria-busy="true">
    <div class="h-8 w-2/3 animate-pulse rounded-lg bg-gray-200 dark:bg-dark-800" />
    <div class="h-48 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-900" />
  </div>

  <template v-else-if="detail">
    <div class="space-y-2.5 border-b border-gray-200 pb-5 dark:border-dark-700">
      <div class="flex flex-wrap items-baseline gap-2.5">
        <h1 class="min-w-0 break-words text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">
          {{ detail.issue.title }}
        </h1>
        <button
          type="button"
          class="rounded px-1 font-mono text-sm text-gray-600 hover:text-primary-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-dark-400 dark:hover:text-primary-400"
          :title="t('admin.issues.actions.copyRef')"
          :aria-label="t('admin.issues.actions.copyRef')"
          @click="copyIssueRef"
        >
          #{{ detail.issue.id }}
        </button>
      </div>
      <div class="flex flex-wrap items-center gap-2.5 text-sm text-gray-500 dark:text-dark-400">
        <span
          class="inline-flex h-7 items-center gap-1.5 rounded-full border px-3 text-xs font-semibold"
          :class="statusStyle[detail.issue.status].badge"
        >
          <span class="h-2 w-2 rounded-full" :class="statusStyle[detail.issue.status].dot" aria-hidden="true" />
          {{ statusLabel(detail.issue.status) }}
        </span>
        <span>{{ kindLabel(detail.issue.kind) }}</span>
        <span v-if="detail.issue.fix_version" class="max-w-full truncate rounded-full bg-primary-500/10 px-2 py-0.5 font-mono text-xs font-medium text-primary-700 dark:text-primary-400">
          {{ detail.issue.fix_version }}
        </span>
      </div>
    </div>
    <div class="grid grid-cols-1 items-start gap-6 xl:grid-cols-[minmax(0,1fr)_272px]">
      <div class="min-w-0 space-y-5">
        <article class="overflow-hidden rounded-2xl border border-gray-200 bg-surface shadow-card dark:border-dark-700 dark:bg-dark-900">
          <div
            class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 bg-gray-50/80 px-4 py-3 text-xs text-gray-500 dark:border-dark-700 dark:bg-dark-800/70 dark:text-dark-400"
          >
            <div class="flex min-w-0 items-center gap-2.5">
              <span class="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-primary-500/10 text-[11px] font-semibold text-primary-700 dark:text-primary-400">
                {{ initialChar(reporterDisplay(detail.issue)) }}
              </span>
              <span class="truncate font-semibold text-gray-900 dark:text-gray-100">{{ reporterDisplay(detail.issue) }}</span>
              <span class="shrink-0" :title="formatDateTime(detail.issue.created_at)">{{ formatRelativeTime(detail.issue.created_at) }}</span>
            </div>
            <span
              v-if="detail.issue.app_version || detail.issue.client_os"
              class="max-w-full truncate rounded-full border border-gray-200 bg-surface px-2.5 py-0.5 font-mono text-[11px] text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
            >
              {{ [detail.issue.app_version, shortOs(detail.issue.client_os)].filter(Boolean).join(' · ') }}
            </span>
          </div>

          <div class="whitespace-pre-wrap break-words px-4 py-4 text-sm leading-relaxed text-gray-700 dark:text-gray-200">
            {{ detail.issue.description || t('admin.issues.detail.noDescription') }}
          </div>

          <IssueAttachments
            v-if="mediaAttachments.length || diagnosticsAttachment"
            :issue-id="issueId"
            :signal="signal"
            :media="mediaAttachments"
            :diagnostics="diagnosticsAttachment"
          />
        </article>
        <div v-if="detail.events.length > 0" class="relative space-y-3.5 pl-8">
          <span
            class="pointer-events-none absolute bottom-2 left-[11px] top-1 w-[1.5px] bg-gray-200 dark:bg-dark-700"
          />

          <div v-for="ev in detail.events" :key="ev.id" class="relative">
            <span
              class="absolute -left-8 top-0.5 grid h-6 w-6 place-items-center rounded-full border bg-surface dark:bg-dark-900"
              :class="
                ev.event_type === 'note' || ev.event_type === 'status_changed'
                  ? 'border-primary-500/40 text-primary-500'
                  : 'border-gray-300 text-gray-400 dark:border-dark-600 dark:text-dark-400'
              "
            >
              <svg
                v-if="ev.event_type === 'note'"
                class="h-3 w-3"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2.1"
              >
                <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
              </svg>
              <span v-else class="h-2 w-2 rounded-full bg-current" />
            </span>
            <div
              v-if="ev.event_type === 'note'"
              class="overflow-hidden rounded-2xl border border-gray-200 bg-surface shadow-card dark:border-dark-700 dark:bg-dark-900"
            >
              <div
                class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 bg-gray-50/80 px-4 py-2.5 text-xs text-gray-500 dark:border-dark-700 dark:bg-dark-800/70 dark:text-dark-400"
              >
                <div class="flex flex-wrap items-center gap-2">
                  <span class="grid h-5 w-5 place-items-center rounded-full bg-primary-500/10 text-[10px] font-semibold text-primary-700 dark:text-primary-400">
                    {{ initialChar(ev.actor_name) }}
                  </span>
                  <span class="font-semibold text-gray-900 dark:text-white">{{ ev.actor_name }}</span>
                  <span class="rounded-full border border-primary-500/30 bg-primary-500/10 px-2 py-0.5 text-[10.5px] font-semibold text-primary-600 dark:text-primary-400">
                    {{ t('admin.issues.detail.internalNote') }}
                  </span>
                  <span
                    v-if="typeof ev.metadata?.fix_version === 'string' && ev.metadata.fix_version"
                    class="rounded-full bg-violet-500/12 px-2 py-0.5 font-mono text-[10.5px] font-semibold text-violet-600 dark:text-violet-400"
                  >
                    → {{ ev.metadata.fix_version }}
                  </span>
                </div>
                <span>{{ formatDateTime(ev.created_at) }}</span>
              </div>
              <div class="whitespace-pre-wrap break-words px-4 py-3.5 text-sm leading-relaxed text-gray-700 dark:text-gray-200">
                {{ ev.body }}
              </div>
            </div>
            <div
              v-else
              class="flex min-h-7 flex-wrap items-center gap-1.5 py-0.5 text-xs text-gray-500 dark:text-dark-400"
            >
              <strong class="font-semibold text-gray-900 dark:text-gray-100">{{ ev.actor_name }}</strong>
              <span>{{ formatAuditEvent(ev) }}</span>
              <span class="opacity-70">· {{ formatDateTime(ev.created_at) }}</span>
            </div>
          </div>
        </div>
        <div class="overflow-hidden rounded-2xl border border-gray-200 bg-surface transition-colors focus-within:border-primary-500 dark:border-dark-700 dark:bg-dark-900">
          <textarea
            v-model="noteDraft"
            rows="4"
            class="w-full resize-y border-0 bg-transparent px-4 py-3.5 text-sm leading-relaxed text-gray-900 outline-none placeholder:text-gray-400 dark:text-gray-100 dark:placeholder:text-dark-400"
            :placeholder="t('admin.issues.detail.notePlaceholder')"
            :aria-label="t('admin.issues.detail.internalNote')"
            :disabled="mutating"
          />
          <div
            class="flex flex-wrap items-center justify-between gap-2.5 border-t border-gray-200 bg-gray-50/80 px-3.5 py-2.5 dark:border-dark-700 dark:bg-dark-800/70"
          >
            <input
              v-model="noteFixVersion"
              type="text"
              class="h-8 w-44 rounded-lg border border-gray-200 bg-surface px-2.5 font-mono text-xs text-gray-900 outline-none focus:border-primary-500 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-100"
              :placeholder="t('admin.issues.detail.fixVersionPlaceholder')"
              :aria-label="t('admin.issues.detail.fixVersionLabel')"
              :disabled="mutating"
            />
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm h-8"
                :disabled="mutating || (!noteDraft.trim() && noteFixVersion.trim() === detail.issue.fix_version)"
                @click="submitNote(false)"
              >
                {{ t('admin.issues.actions.addNote') }}
              </button>
              <button
                type="button"
                class="btn btn-primary btn-sm h-8"
                :disabled="mutating"
                @click="submitNote(true)"
              >
                {{
                  detail.issue.status === 'resolved'
                    ? t('admin.issues.actions.noteAndClose')
                    : t('admin.issues.actions.noteAndResolve')
                }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <aside class="min-w-0 divide-y divide-gray-200 dark:divide-dark-700 xl:sticky xl:top-20">
        <div class="space-y-2.5 pb-4">
          <div class="text-xs font-semibold text-gray-500 dark:text-dark-400">
            {{ t('admin.issues.detail.statusLabel') }}
          </div>
          <div class="grid grid-cols-2 gap-1.5">
            <button
              v-for="st in issueStatuses"
              :key="st"
              type="button"
              class="inline-flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs font-medium transition-colors"
              :class="
                detail.issue.status === st
                  ? statusStyle[st].badge
                  : 'border-gray-200 bg-surface text-gray-600 hover:border-gray-300 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300'
              "
              :disabled="mutating"
              :aria-pressed="detail.issue.status === st"
              @click="patchIssue({ status: st })"
            >
              <span class="h-2 w-2 rounded-full" :class="statusStyle[st].dot" />
              {{ statusLabel(st) }}
            </button>
          </div>
        </div>
        <div class="space-y-2.5 py-4">
          <div class="text-xs font-semibold text-gray-500 dark:text-dark-400">
            {{ t('admin.issues.detail.severityLabel') }}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="sev in issueSeverities"
              :key="sev"
              type="button"
              class="inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-medium transition-colors"
              :class="
                detail.issue.severity === sev
                  ? severityStyle[sev] + ' border-current/25'
                  : 'border-gray-200 bg-surface text-gray-500 hover:text-gray-800 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-400'
              "
              :disabled="mutating"
              :aria-pressed="detail.issue.severity === sev"
              @click="patchIssue({ severity: sev })"
            >
              {{ severityLabel(sev) }}
            </button>
          </div>
        </div>
        <div class="space-y-2 py-4">
          <div class="text-xs font-semibold text-gray-500 dark:text-dark-400">
            {{ t('admin.issues.detail.assigneeLabel') }}
          </div>
          <form class="flex gap-1.5" @submit.prevent="saveAssignee">
            <input
              v-model="assigneeDraft"
              type="text"
              class="input h-8 min-w-0 px-2.5 py-1 text-xs"
              :placeholder="t('admin.issues.detail.assigneePlaceholder')"
              :aria-label="t('admin.issues.detail.assigneeLabel')"
              :disabled="mutating"
            />
            <button
              type="submit"
              class="btn btn-secondary btn-sm h-8 shrink-0 px-2.5"
              :disabled="mutating || assigneeDraft.trim() === detail.issue.assignee_name"
            >
              {{ t('admin.issues.actions.save') }}
            </button>
          </form>
        </div>
        <div class="space-y-2.5 py-4">
          <div class="text-xs font-semibold text-gray-500 dark:text-dark-400">
            {{ t('admin.issues.detail.labelsLabel') }}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="lbl in availableLabels"
              :key="lbl"
              type="button"
              class="inline-flex h-6 items-center rounded-full border px-2.5 text-[11.5px] font-medium transition-colors"
              :class="
                detail.issue.labels.includes(lbl)
                  ? 'border-primary-500/35 bg-primary-500/12 text-primary-600 dark:text-primary-400'
                  : 'border-gray-200 bg-surface text-gray-500 hover:text-gray-900 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-400'
              "
              :disabled="mutating"
              :aria-pressed="detail.issue.labels.includes(lbl)"
              @click="toggleLabel(lbl)"
            >
              <Icon :name="detail.issue.labels.includes(lbl) ? 'check' : 'plus'" size="xs" class="mr-1" aria-hidden="true" />{{ lbl }}
            </button>
          </div>
          <form class="flex gap-1.5" @submit.prevent="addCustomLabel">
            <input
              v-model="customLabelDraft"
              type="text"
              class="input h-8 min-w-0 px-2.5 py-1 text-xs"
              :placeholder="t('admin.issues.detail.customLabelPlaceholder')"
              :aria-label="t('admin.issues.detail.customLabelPlaceholder')"
              :disabled="mutating"
            />
            <button
              type="submit"
              class="btn btn-secondary btn-sm h-8 shrink-0 px-2.5"
              :disabled="mutating || !customLabelDraft.trim()"
              :aria-label="t('admin.issues.actions.addLabel')"
            >
              <Icon name="plus" size="sm" aria-hidden="true" />
            </button>
          </form>
        </div>
        <div class="space-y-2.5 py-4">
          <div class="flex items-center justify-between text-xs font-semibold text-gray-500 dark:text-dark-400">
            <span>{{ t('admin.issues.detail.envLabel') }}</span>
            <button
              type="button"
              class="font-medium text-primary-600 hover:underline dark:text-primary-400"
              @click="copyEnvSnapshot"
            >
              {{ t('admin.issues.actions.copyEnv') }}
            </button>
          </div>
          <div class="space-y-1.5 text-xs">
            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.issues.detail.envVersion') }}</span>
              <span class="min-w-0 break-words text-right font-mono text-gray-900 dark:text-gray-100">{{ detail.issue.app_version || '—' }}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.issues.detail.envOs') }}</span>
              <span class="min-w-0 break-words text-right font-mono text-gray-900 dark:text-gray-100">{{ detail.issue.client_os || '—' }}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.issues.detail.envArch') }}</span>
              <span class="min-w-0 break-words text-right font-mono text-gray-900 dark:text-gray-100">{{ detail.issue.client_arch || '—' }}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.issues.detail.envRoute') }}</span>
              <span class="min-w-0 break-all text-right font-mono text-gray-900 dark:text-gray-100">{{ detail.issue.client_route || '—' }}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.issues.detail.envLocale') }}</span>
              <span class="min-w-0 break-words text-right font-mono text-gray-900 dark:text-gray-100">{{ detail.issue.client_locale || '—' }}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.issues.detail.envDiagnostics') }}</span>
              <span class="min-w-0 break-words text-right font-mono text-gray-900 dark:text-gray-100">
                {{ diagnosticsAttachment ? formatBytes(diagnosticsAttachment.size_bytes) : t('admin.issues.detail.notAttached') }}
              </span>
            </div>
          </div>
        </div>
        <div class="space-y-2.5 pt-4">
          <div class="text-xs font-semibold text-gray-500 dark:text-dark-400">
            {{ t('admin.issues.detail.reporterLabel') }}
          </div>
          <div class="flex items-center gap-2.5">
            <span class="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-primary-500/10 text-xs font-semibold text-primary-700 dark:text-primary-400">
              {{ initialChar(reporterDisplay(detail.issue)) }}
            </span>
            <div class="min-w-0">
              <div class="truncate text-xs font-semibold text-gray-900 dark:text-gray-100">
                {{ reporterDisplay(detail.issue) }}
              </div>
              <div class="font-mono text-[11px] text-gray-500 dark:text-dark-400">
                {{ detail.issue.reporter_user_id ? `uid_${detail.issue.reporter_user_id}` : '—' }}
              </div>
            </div>
          </div>
          <button
            v-if="detail.issue.reporter_email"
            type="button"
            class="text-left text-xs font-medium text-primary-600 hover:underline dark:text-primary-400"
            @click="emit('filterByReporter', detail.issue.reporter_email)"
          >
            {{ t('admin.issues.detail.viewReporterIssues', { count: detail.reporter_issue_count }) }}
          </button>
        </div>
      </aside>
    </div>
  </template>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { issuesAPI } from '@/api/admin'
import type { DesktopIssue, DesktopIssueDetail, DesktopIssueEvent, DesktopIssueStatus } from '@/api/admin/issues'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatBytes, formatDateTime, formatRelativeTime } from '@/utils/format'
import IssueAttachments from './IssueAttachments.vue'
import { initialChar, issueSeverities, issueStatuses, reporterDisplay, severityStyle, shortOs, statusStyle } from './presentation'

const { issueId } = defineProps<{ issueId: number }>()
const emit = defineEmits<{ filterByReporter: [email: string] }>()
const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const appStore = useAppStore()

// The keyed route outlet gives each visit its own requests and drafts, even for the same ID.
const controller = new AbortController()
const { signal } = controller
onBeforeUnmount(() => controller.abort())

const detail = shallowRef<DesktopIssueDetail>()
const loadError = ref('')
const mutating = ref(false)
const assigneeDraft = ref('')
const customLabelDraft = ref('')
const noteDraft = ref('')
const noteFixVersion = ref('')

const presetLabels = ['bug', 'feature', 'experience', 'runtime', 'ui', 'plugin', 'network', 'performance', 'diagnostics']
const mediaAttachments = computed(() => detail.value?.attachments.filter(att => att.category !== 'diagnostics') ?? [])
const diagnosticsAttachment = computed(() => detail.value?.attachments.find(att => att.category === 'diagnostics'))
const availableLabels = computed(() => [...new Set([...presetLabels, ...(detail.value?.issue.labels ?? [])])])

const statusLabel = (status: string) => t(`admin.issues.status.${status}`)
const kindLabel = (kind: string) => t(`admin.issues.kind.${kind}`)
const severityLabel = (severity: string) => t(`admin.issues.severity.${severity}`)

async function loadDetail() {
  loadError.value = ''
  try {
    const result = await issuesAPI.getById(issueId, { signal })
    if (signal.aborted) return
    detail.value = result
    assigneeDraft.value = result.issue.assignee_name
    noteFixVersion.value = result.issue.fix_version
  } catch (error) {
    if (!signal.aborted) loadError.value = extractApiErrorMessage(error, t('admin.issues.detail.loadFailed'))
  }
}

async function mutateIssue(request: () => Promise<DesktopIssueDetail>, onSaved?: (issue: DesktopIssue) => void) {
  const previous = detail.value?.issue
  if (!previous || mutating.value) return
  mutating.value = true
  try {
    const result = await request()
    if (signal.aborted) return
    detail.value = result
    if (assigneeDraft.value === previous.assignee_name) assigneeDraft.value = result.issue.assignee_name
    if (noteFixVersion.value === previous.fix_version) noteFixVersion.value = result.issue.fix_version
    onSaved?.(result.issue)
  } catch (error) {
    if (!signal.aborted) appStore.showError(extractApiErrorMessage(error, t('admin.issues.detail.updateFailed')))
  } finally {
    if (!signal.aborted) mutating.value = false
  }
}

function patchIssue(payload: Parameters<typeof issuesAPI.update>[1], onSaved?: (issue: DesktopIssue) => void) {
  return mutateIssue(() => issuesAPI.update(issueId, payload, { signal }), onSaved)
}

function saveAssignee() {
  const name = assigneeDraft.value.trim()
  if (name !== detail.value?.issue.assignee_name) {
    void patchIssue({ assignee_name: name }, issue => { assigneeDraft.value = issue.assignee_name })
  }
}

function toggleLabel(label: string) {
  const labels = detail.value!.issue.labels
  void patchIssue({ labels: labels.includes(label) ? labels.filter(item => item !== label) : [...labels, label] })
}

function addCustomLabel() {
  const label = customLabelDraft.value.trim().toLowerCase()
  const labels = detail.value!.issue.labels
  if (!label || labels.includes(label)) return
  void patchIssue({ labels: [...labels, label] }, () => { customLabelDraft.value = '' })
}

function submitNote(withStatusTransition: boolean) {
  const issue = detail.value!.issue
  const body = noteDraft.value.trim()
  const version = noteFixVersion.value.trim()
  const fix_version = version !== issue.fix_version ? version : undefined
  const status: DesktopIssueStatus | undefined = withStatusTransition
    ? issue.status === 'resolved' ? 'closed' : 'resolved'
    : undefined
  if (!body && !status && fix_version === undefined) return

  void mutateIssue(() => issuesAPI.addNote(issueId, { body, status, fix_version }, { signal }), saved => {
    noteDraft.value = ''
    noteFixVersion.value = saved.fix_version
  })
}

function copyIssueRef() {
  const issue = detail.value!.issue
  void copyToClipboard(`#${issue.id} ${issue.title}`, t('admin.issues.actions.copiedRef', { id: issue.id }))
}

function copyEnvSnapshot() {
  const issue = detail.value!.issue
  const summary = [
    issue.app_version,
    [issue.client_os, issue.client_arch].filter(Boolean).join(' '),
    issue.client_route,
    issue.client_locale
  ].filter(Boolean).join(' | ')
  void copyToClipboard(summary, t('admin.issues.actions.copiedEnv'))
}

function formatAuditEvent(ev: DesktopIssueEvent): string {
  const meta = ev.metadata ?? {}
  switch (ev.event_type) {
    case 'created':
      return meta.has_diagnostics ? t('admin.issues.timeline.createdWithDiag') : t('admin.issues.timeline.created')
    case 'status_changed': {
      const from = statusLabel(String(meta.from ?? ''))
      const to = statusLabel(String(meta.to ?? ''))
      return meta.batch
        ? t('admin.issues.timeline.statusChangedBatch', { from, to })
        : t('admin.issues.timeline.statusChanged', { from, to })
    }
    case 'severity_changed':
      return t('admin.issues.timeline.severityChanged', {
        from: severityLabel(String(meta.from ?? '')),
        to: severityLabel(String(meta.to ?? ''))
      })
    case 'assignee_changed': {
      const to = String(meta.to ?? '').trim()
      return to ? t('admin.issues.timeline.assigneeChanged', { to }) : t('admin.issues.timeline.assigneeCleared')
    }
    case 'fix_version_changed': {
      const to = String(meta.to ?? '').trim()
      return to ? t('admin.issues.timeline.fixVersionChanged', { to }) : t('admin.issues.timeline.fixVersionCleared')
    }
    case 'labels_changed':
      return t('admin.issues.timeline.labelsUpdated')
    default:
      return ev.event_type
  }
}

void loadDetail()
</script>
