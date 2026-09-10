<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="flex flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('resourcePackages.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('resourcePackages.description') }}</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPackages">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              {{ t('common.refresh') }}
            </button>
            <button type="button" class="btn btn-primary" :disabled="uploading" @click="openFilePicker">
              <Icon name="upload" size="sm" />
              {{ uploading ? t('common.processing') : t('resourcePackages.actions.upload') }}
            </button>
            <input
              ref="fileInput"
              type="file"
              class="hidden"
              accept=".matcha-skillpkg,.matcha-agentpkg"
              @change="uploadSelectedPackage"
            />
          </div>
        </div>
      </template>

      <template #filters>
        <div class="flex flex-col justify-between gap-4 lg:flex-row lg:items-center">
          <div class="inline-flex rounded-lg border border-gray-200 bg-white p-1 dark:border-dark-700 dark:bg-dark-800">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              type="button"
              class="rounded-md px-4 py-2 text-sm font-medium transition-colors"
              :class="activeTab === tab.value ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
              @click="setTab(tab.value)"
            >
              {{ tab.label }}
            </button>
          </div>

          <div class="flex flex-1 flex-wrap items-center justify-end gap-3">
            <SearchInput
              v-model="searchQuery"
              :placeholder="t('resourcePackages.filters.searchPlaceholder')"
              class="w-full sm:w-72"
              @search="onFilterChange"
            />
            <Select
              :model-value="packageType"
              :options="packageTypeOptions"
              class="w-44"
              @update:model-value="onTypeChange"
            />
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="packages"
          :loading="loading"
          row-key="packageVersionId"
          :sticky-first-column="false"
          :expandable-actions="false"
        >
          <template #cell-name="{ row }">
            <div class="max-w-sm">
              <div class="font-medium text-gray-900 dark:text-white">{{ packageTitle(row) }}</div>
              <div class="mt-1 truncate font-mono text-xs text-gray-500 dark:text-gray-400">{{ row.name }}</div>
              <p v-if="row.description" class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">{{ row.description }}</p>
            </div>
          </template>

          <template #cell-packageType="{ value }">
            <span class="rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">
              {{ typeLabel(value) }}
            </span>
          </template>

          <template #cell-version="{ value }">
            <span class="font-mono text-xs text-gray-600 dark:text-gray-300">{{ shortVersion(value) }}</span>
          </template>

          <template #cell-status="{ value }">
            <span class="rounded-full px-2 py-1 text-xs font-medium" :class="statusClass(value)">
              {{ statusLabel(value) }}
            </span>
          </template>

          <template #cell-entitlementStatus="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ entitlementLabel(value) }}</span>
          </template>

          <template #cell-metering="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ meteringLabel(row.meteringBinding) }}</span>
          </template>

          <template #cell-updatedAt="{ row }">
            <span class="whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">{{ formatDate(row.updatedAt || row.createdAt) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex gap-2">
              <button
                v-if="activeTab === 'mine' && row.status === 'draft'"
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="publishingId !== null"
                @click.stop="publish(row)"
              >
                {{ publishingId === row.packageVersionId ? t('common.processing') : t('resourcePackages.actions.publish') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="!row.downloadable || downloadingId === row.packageVersionId"
                @click.stop="download(row)"
              >
                <Icon name="download" size="sm" />
                {{ t('resourcePackages.actions.download') }}
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('resourcePackages.empty.title')"
              :description="t('resourcePackages.empty.description')"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import packagesAPI, {
  type PackageListParams,
  type PackageMeteringBinding,
  type ResourcePackageType,
  type ResourcePackageVersion,
} from '@/api/packages'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode, extractApiErrorMetadata } from '@/utils/apiError'

const { t, te } = useI18n()
const appStore = useAppStore()

type PackageTab = 'market' | 'mine'

const activeTab = ref<PackageTab>('market')
const packages = ref<ResourcePackageVersion[]>([])
const loading = ref(false)
const uploading = ref(false)
const publishingId = ref<string | null>(null)
const downloadingId = ref<string | null>(null)
const searchQuery = ref('')
const packageType = ref<ResourcePackageType | ''>('')
const fileInput = ref<HTMLInputElement | null>(null)
let abortController: AbortController | null = null

const pagination = ref({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0,
})

const tabs = computed(() => [
  { value: 'market' as const, label: t('resourcePackages.tabs.market') },
  { value: 'mine' as const, label: t('resourcePackages.tabs.mine') },
])

const packageTypeOptions = computed(() => [
  { value: '', label: t('resourcePackages.filters.allTypes') },
  { value: 'agent', label: t('resourcePackages.filters.agent') },
  { value: 'skill', label: t('resourcePackages.filters.skill') },
])

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('resourcePackages.columns.name') },
  { key: 'packageType', label: t('resourcePackages.columns.type') },
  { key: 'version', label: t('resourcePackages.columns.version') },
  { key: 'status', label: t('resourcePackages.columns.status') },
  { key: 'entitlementStatus', label: t('resourcePackages.columns.entitlement') },
  { key: 'metering', label: t('resourcePackages.columns.metering') },
  { key: 'updatedAt', label: t('resourcePackages.columns.updatedAt') },
  { key: 'actions', label: t('resourcePackages.columns.actions') },
])

function queryParams(): PackageListParams {
  const params: PackageListParams = {
    page: pagination.value.page,
    page_size: pagination.value.page_size,
  }
  const search = searchQuery.value.trim()
  if (search) params.search = search
  if (packageType.value) params.packageType = packageType.value
  return params
}

function isAbortError(error: unknown) {
  if (!error || typeof error !== 'object') return false
  const { name, code } = error as { name?: string; code?: string }
  return name === 'AbortError' || code === 'ERR_CANCELED'
}

function packageErrorMessage(error: unknown, fallbackKey: string): string {
  const code = extractApiErrorCode(error)
  if (code) {
    const key = `resourcePackages.errors.${code}`
    if (te(key)) {
      return t(key, extractApiErrorMetadata(error) ?? {})
    }
  }
  return t(fallbackKey)
}

async function loadPackages() {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  loading.value = true
  try {
    const response = activeTab.value === 'mine'
      ? await packagesAPI.listMine(queryParams(), { signal: controller.signal })
      : await packagesAPI.listMarket(queryParams(), { signal: controller.signal })
    if (controller.signal.aborted) return
    packages.value = response.items
    pagination.value.total = response.total
    pagination.value.pages = response.pages
  } catch (error) {
    if (!isAbortError(error)) {
      appStore.showError(packageErrorMessage(error, 'resourcePackages.errors.failedToLoad'))
    }
  } finally {
    if (abortController === controller) {
      loading.value = false
    }
  }
}

function onFilterChange() {
  pagination.value.page = 1
  loadPackages()
}

function onTypeChange(value: string | number | boolean | null) {
  packageType.value = value === 'agent' || value === 'skill' ? value : ''
  onFilterChange()
}

function setTab(tab: PackageTab) {
  if (activeTab.value === tab) return
  activeTab.value = tab
  pagination.value.page = 1
  loadPackages()
}

function handlePageChange(page: number) {
  pagination.value.page = page
  loadPackages()
}

function handlePageSizeChange(pageSize: number) {
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  loadPackages()
}

function openFilePicker() {
  fileInput.value?.click()
}

async function uploadSelectedPackage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return

  uploading.value = true
  try {
    await packagesAPI.uploadPackage(file)
    appStore.showSuccess(t('resourcePackages.messages.uploaded'))
    activeTab.value = 'mine'
    pagination.value.page = 1
    await loadPackages()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'resourcePackages.errors.failedToUpload'))
  } finally {
    uploading.value = false
  }
}

async function publish(pkg: ResourcePackageVersion) {
  if (activeTab.value !== 'mine' || pkg.status !== 'draft' || publishingId.value !== null) return
  publishingId.value = pkg.packageVersionId
  try {
    const published = await packagesAPI.publishPackageVersion(pkg.packageVersionId)
    packages.value = packages.value.map((item) => item.packageVersionId === published.packageVersionId ? published : item)
    appStore.showSuccess(t('resourcePackages.messages.published'))
    await loadPackages()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'resourcePackages.errors.failedToPublish'))
  } finally {
    publishingId.value = null
  }
}

async function download(pkg: ResourcePackageVersion) {
  downloadingId.value = pkg.packageVersionId
  try {
    const blob = await packagesAPI.downloadPackage(pkg.packageVersionId)
    saveBlob(blob, packageFilename(pkg))
    appStore.showSuccess(t('resourcePackages.messages.downloaded'))
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'resourcePackages.errors.failedToDownload'))
  } finally {
    downloadingId.value = null
  }
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function packageTitle(pkg: ResourcePackageVersion) {
  return pkg.displayName || pkg.name
}

function packageFilename(pkg: ResourcePackageVersion) {
  const ext = pkg.packageType === 'agent' ? '.matcha-agentpkg' : '.matcha-skillpkg'
  const base = packageTitle(pkg).replace(/[^a-zA-Z0-9._-]+/g, '-').replace(/^-+|-+$/g, '') || 'matcha-package'
  return base.endsWith(ext) ? base : `${base}${ext}`
}

function typeLabel(value: string) {
  if (value === 'agent') return t('resourcePackages.filters.agent')
  if (value === 'skill') return t('resourcePackages.filters.skill')
  return value || t('common.unknown')
}

function statusLabel(value: string) {
  return value ? t(`resourcePackages.status.${value}`, value) : t('common.unknown')
}

function entitlementLabel(value?: string) {
  return value ? t(`resourcePackages.entitlement.${value}`, value) : t('common.notAvailable')
}

function statusClass(value: string) {
  if (value === 'published') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (value === 'disabled') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  if (value === 'yanked') return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

function meteringLabel(binding?: PackageMeteringBinding) {
  if (!binding || !binding.amount) return t('resourcePackages.metering.free')
  const unit = binding.unit ? t(`resourcePackages.metering.${binding.unit}`, binding.unit) : ''
  return `${binding.currency || 'USD'} ${binding.amount}${unit ? ` / ${unit}` : ''}`
}

function shortVersion(value?: string) {
  if (!value) return '-'
  return value.length > 16 ? `${value.slice(0, 12)}…` : value
}

function formatDate(value?: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

onMounted(loadPackages)
onBeforeUnmount(() => abortController?.abort())
</script>
