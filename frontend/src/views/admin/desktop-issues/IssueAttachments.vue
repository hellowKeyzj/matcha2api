<template>
  <div class="space-y-3 px-4 pb-4">
    <div v-if="media.length" class="grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3">
      <button
        v-for="attachment in media"
        :key="attachment.id"
        type="button"
        class="group relative aspect-[16/10] min-w-0 overflow-hidden rounded-xl border border-gray-200 bg-gray-100 text-left hover:border-primary-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-400"
        :aria-label="t('admin.issues.previewAttachment', { name: attachment.file_name })"
        :title="attachment.file_name"
        @click="openPreview(attachment)"
      >
        <img
          v-if="thumbnails.get(attachment.id)"
          :src="thumbnails.get(attachment.id) ?? undefined"
          alt=""
          class="h-full w-full object-cover"
        />
        <span v-else class="absolute inset-0 flex items-center justify-center px-2 pb-5 text-center text-xs text-gray-600 dark:text-dark-300">
          <span v-if="attachment.category === 'video'" class="grid h-8 w-8 place-items-center rounded-full bg-primary-500/10 text-primary-700 dark:text-primary-400">
            <Icon name="play" size="sm" aria-hidden="true" />
          </span>
          <span v-else>{{ thumbnails.get(attachment.id) === null ? t('admin.issues.detail.mediaLoadFailed') : t('common.loading') }}</span>
        </span>
        <span
          class="absolute inset-x-0 bottom-0 flex items-center justify-between gap-2 px-2 py-1.5 text-[11px]"
          :class="thumbnails.get(attachment.id) ? 'bg-black/65 text-white' : 'bg-gray-50/90 text-gray-700 dark:bg-dark-900/90 dark:text-gray-200'"
        >
          <span class="truncate">{{ attachment.file_name }}</span>
          <span class="shrink-0 font-mono">{{ formatBytes(attachment.size_bytes) }}</span>
        </span>
      </button>
    </div>

    <div v-if="diagnostics" class="overflow-hidden rounded-xl border border-primary-500/25 bg-primary-500/5">
      <div class="flex flex-wrap items-center justify-between gap-3 p-3">
        <div class="flex min-w-0 items-center gap-2">
          <Icon name="document" size="lg" class="shrink-0 text-primary-600 dark:text-primary-400" aria-hidden="true" />
          <div class="min-w-0">
            <div class="truncate text-sm font-medium text-gray-900 dark:text-white" :title="diagnostics.file_name">
              {{ diagnostics.file_name }}
            </div>
            <div class="flex flex-wrap items-center gap-x-2 text-xs text-gray-600 dark:text-dark-400">
              <span>{{ formatBytes(diagnostics.size_bytes) }}</span>
              <button
                type="button"
                class="inline-flex items-center gap-1 rounded hover:text-primary-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:hover:text-primary-400"
                :title="t('admin.issues.actions.copySha')"
                :aria-label="t('admin.issues.actions.copySha')"
                @click="copyToClipboard(diagnostics.sha256, t('admin.issues.actions.copiedSha'))"
              >
                SHA-256 <Icon name="copy" size="xs" aria-hidden="true" />
              </button>
            </div>
          </div>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :aria-expanded="manifestOpen"
            @click="toggleManifest(diagnostics.id)"
          >
            {{ t('admin.issues.actions.inspectManifest') }}
          </button>
          <button type="button" class="btn btn-primary btn-sm" :disabled="downloading" @click="downloadAttachment(diagnostics)">
            {{ t('admin.issues.actions.downloadDiagnostics') }}
          </button>
        </div>
      </div>

      <div v-if="manifestOpen" class="border-t border-primary-500/20 bg-surface px-3 py-2.5 text-xs text-gray-700 dark:bg-dark-900 dark:text-gray-200" :aria-busy="manifestLoading">
        <p v-if="manifestLoading">{{ t('admin.issues.actions.loadingManifest') }}</p>
        <p v-else-if="manifestError" role="alert">{{ manifestError }}</p>
        <p v-else-if="!manifestEntries?.length">{{ t('admin.issues.actions.manifestEmpty') }}</p>
        <ul v-else class="max-h-56 space-y-1 overflow-y-auto font-mono" :aria-label="t('admin.issues.actions.inspectManifest')" tabindex="0">
          <li v-for="(entry, index) in manifestEntries" :key="index" class="flex justify-between gap-3">
            <span class="min-w-0 break-all">{{ entry.name }}</span>
            <span class="shrink-0 text-gray-500 dark:text-dark-400">{{ formatBytes(entry.size_bytes) }}</span>
          </li>
        </ul>
      </div>
    </div>

    <BaseDialog
      v-if="media.length"
      :show="!!activeAttachment"
      :title="activeAttachment?.file_name ?? ''"
      width="wide"
      panel-class="[overflow-wrap:anywhere]"
      close-on-click-outside
      @close="activeAttachment = undefined"
    >
      <div v-if="activeAttachment" class="flex items-center justify-center rounded-lg bg-gray-100 dark:bg-dark-900">
        <p v-if="previewFailed" class="px-4 py-10 text-sm text-gray-600 dark:text-dark-300" role="alert">
          {{ t('admin.issues.detail.mediaLoadFailed') }}
        </p>
        <img v-else-if="activeAttachment.category === 'image' && previewUrl" :src="previewUrl" :alt="activeAttachment.file_name" class="max-h-[65vh] max-w-full object-contain" />
        <video
          v-else-if="previewUrl"
          :src="previewUrl"
          :aria-label="activeAttachment.file_name"
          controls
          autoplay
          playsinline
          class="max-h-[65vh] max-w-full"
          @error="recordingFailed = true"
        />
        <p v-else class="px-4 py-10 text-sm text-gray-600 dark:text-dark-300" role="status">{{ t('common.loading') }}</p>
      </div>
      <template #footer>
        <button v-if="activeAttachment" type="button" class="btn btn-secondary btn-sm" :disabled="downloading" @click="downloadAttachment(activeAttachment)">
          <Icon name="download" size="sm" aria-hidden="true" />{{ t('admin.issues.actions.download') }}
        </button>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowReactive, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useObjectUrl } from '@vueuse/core'
import { saveAs } from 'file-saver'
import { issuesAPI } from '@/api/admin'
import type { DesktopIssueAttachment, DesktopIssueManifestEntry } from '@/api/admin/issues'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatBytes } from '@/utils/format'

const { issueId, signal, media, diagnostics } = defineProps<{
  issueId: number
  signal: AbortSignal
  media: DesktopIssueAttachment[]
  diagnostics?: DesktopIssueAttachment
}>()
const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const appStore = useAppStore()

const activeAttachment = shallowRef<DesktopIssueAttachment>()
const thumbnails = shallowReactive(new Map<number, string | null>())
const recording = shallowRef<Blob>()
const recordingFailed = ref(false)
const recordingUrl = useObjectUrl(recording)
const downloading = ref(false)
const manifestOpen = ref(false)
const manifestLoading = ref(false)
const manifestError = ref('')
const manifestEntries = shallowRef<DesktopIssueManifestEntry[]>()

const previewUrl = computed(() => {
  const attachment = activeAttachment.value
  if (!attachment) return undefined
  return attachment.category === 'video' ? recordingUrl.value : thumbnails.get(attachment.id)
})
const previewFailed = computed(() => recordingFailed.value || previewUrl.value === null)

async function loadThumbnail(id: number) {
  thumbnails.delete(id)
  try {
    const blob = await issuesAPI.fetchAttachmentBlob(issueId, id, { signal })
    if (!signal.aborted) thumbnails.set(id, URL.createObjectURL(blob))
  } catch {
    if (!signal.aborted) thumbnails.set(id, null)
  }
}

function openPreview(attachment: DesktopIssueAttachment) {
  activeAttachment.value = attachment
  if (thumbnails.get(attachment.id) === null) void loadThumbnail(attachment.id)
}

// Fetch a recording only while its preview is open; screenshots are shared with the gallery.
watch(activeAttachment, async (attachment, _, onCleanup) => {
  recording.value = undefined
  recordingFailed.value = false
  if (attachment?.category !== 'video') return
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  try {
    const blob = await issuesAPI.fetchAttachmentBlob(issueId, attachment.id, { signal: controller.signal })
    if (!controller.signal.aborted) recording.value = blob
  } catch {
    if (!controller.signal.aborted) recordingFailed.value = true
  }
}, { flush: 'sync' })

async function downloadAttachment(attachment: DesktopIssueAttachment) {
  if (downloading.value) return
  downloading.value = true
  try {
    const blob = await issuesAPI.fetchAttachmentBlob(issueId, attachment.id, { download: true, signal })
    if (!signal.aborted) saveAs(blob, attachment.file_name)
  } catch (error) {
    if (!signal.aborted) appStore.showError(extractApiErrorMessage(error, t('admin.issues.detail.downloadFailed')))
  } finally {
    if (!signal.aborted) downloading.value = false
  }
}

async function toggleManifest(attachmentId: number) {
  manifestOpen.value = !manifestOpen.value
  if (!manifestOpen.value || manifestEntries.value || manifestLoading.value) return
  manifestLoading.value = true
  manifestError.value = ''
  try {
    const result = await issuesAPI.getDiagnosticsManifest(issueId, attachmentId, { signal })
    if (!signal.aborted) manifestEntries.value = result.entries
  } catch (error) {
    if (!signal.aborted) manifestError.value = extractApiErrorMessage(error, t('admin.issues.detail.manifestLoadFailed'))
  } finally {
    if (!signal.aborted) manifestLoading.value = false
  }
}

for (const attachment of media) {
  if (attachment.category === 'image') void loadThumbnail(attachment.id)
}
onBeforeUnmount(() => {
  for (const url of thumbnails.values()) if (url) URL.revokeObjectURL(url)
})
</script>
