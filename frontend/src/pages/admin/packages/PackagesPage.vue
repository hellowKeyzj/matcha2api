<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.packages.description') }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadAll">
            <Icon name="refresh" size="sm" />
            {{ t('admin.packages.actions.refresh') }}
          </button>
          <button type="button" class="btn btn-secondary" @click="openStorageDialog">
            <Icon name="cog" size="sm" />
            {{ t('admin.packages.actions.storageConfig') }}
          </button>
          <button type="button" class="btn btn-primary" @click="openCreateDialog">
            <Icon name="plus" size="sm" />
            {{ t('admin.packages.actions.create') }}
          </button>
        </div>
      </section>

      <div class="flex flex-wrap gap-3">
        <input
          v-model="filters.search"
          class="input max-w-sm"
          :placeholder="t('admin.packages.filters.searchPlaceholder')"
          @input="loadPackages"
        />
        <select v-model="filters.status" class="input w-44" @change="loadPackages">
          <option value="">{{ t('admin.packages.filters.allStatuses') }}</option>
          <option v-for="status in packageStatuses" :key="status" :value="status">{{ statusLabel(status) }}</option>
        </select>
      </div>

      <div class="card flex flex-col gap-4 p-4 md:flex-row md:items-center md:justify-between">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <p class="font-medium text-gray-900 dark:text-white">{{ t('admin.packages.cloudKey.title') }}</p>
            <span class="rounded px-2 py-0.5 text-xs font-medium" :class="cloudKeyStatus.configured ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'">
              {{ cloudKeyStatus.configured ? t('admin.packages.cloudKey.configured') : t('admin.packages.cloudKey.notConfigured') }}
            </span>
          </div>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.packages.cloudKey.description') }}</p>
          <p v-if="cloudKeyStatus.configured" class="mt-2 break-all font-mono text-xs text-gray-500">
            {{ cloudKeyStatus.key_id }} · {{ cloudKeyStatus.algorithm }} · {{ sourceLabel(cloudKeyStatus.source) }}
          </p>
        </div>
        <button type="button" class="btn btn-primary shrink-0" :disabled="cloudKeyStatus.configured || loadingCloudKey || generatingCloudKey" @click="generateCloudKey">
          <Icon name="key" size="sm" />
          {{ generatingCloudKey ? t('admin.packages.cloudKey.generating') : t('admin.packages.cloudKey.generate') }}
        </button>
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs text-gray-500">{{ t('admin.packages.summary.total') }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ packages.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs text-gray-500">{{ t('admin.packages.summary.pendingReview') }}</p>
          <p class="mt-2 text-2xl font-semibold text-amber-600">{{ statusCount('pending_review') }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs text-gray-500">{{ t('admin.packages.summary.approved') }}</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600">{{ statusCount('approved') }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs text-gray-500">{{ t('admin.packages.summary.downloads') }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ totalDownloads }}</p>
        </div>
      </div>

      <div v-if="loading" class="flex min-h-48 items-center justify-center text-sm text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="packages.length === 0" class="flex min-h-56 flex-col items-center justify-center border border-dashed border-gray-300 px-6 text-center dark:border-dark-600">
        <Icon name="cube" size="xl" class="text-gray-400" />
        <p class="mt-3 font-medium text-gray-800 dark:text-gray-200">{{ t('admin.packages.empty.title') }}</p>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.packages.empty.description') }}</p>
      </div>

      <div v-else class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <article v-for="item in packages" :key="item.id" class="card overflow-hidden border border-gray-200 dark:border-dark-700">
          <div class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 p-5 dark:border-dark-700">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ item.name }}</h3>
                <span v-if="item.version" class="font-mono text-xs text-gray-500">v{{ item.version }}</span>
                <span class="rounded px-2 py-0.5 text-xs font-medium" :class="statusClass(item.status)">{{ statusLabel(item.status) }}</span>
              </div>
              <p class="mt-1 text-xs text-gray-500">#{{ item.id }} · {{ formatDate(item.updated_at) }}</p>
              <p v-if="item.description" class="mt-2 text-sm text-gray-600 dark:text-gray-300">{{ item.description }}</p>
            </div>
            <div class="flex flex-wrap gap-2">
              <button type="button" class="btn btn-secondary btn-sm" @click="openEditDialog(item)">
                <Icon name="edit" size="sm" />
                {{ t('admin.packages.actions.edit') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="openAuditDialog(item)">
                <Icon name="chartBar" size="sm" />
                {{ t('admin.packages.actions.audit') }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-4 p-5 text-sm md:grid-cols-2">
            <div>
              <p class="text-xs font-medium uppercase text-gray-500">{{ t('admin.packages.fields.planBindings') }}</p>
              <p class="mt-2 text-gray-800 dark:text-gray-200">{{ listText(item.plan_bindings) }}</p>
            </div>
            <div>
              <p class="text-xs font-medium uppercase text-gray-500">{{ t('admin.packages.fields.permissionBindings') }}</p>
              <p class="mt-2 text-gray-800 dark:text-gray-200">{{ listText(item.permission_bindings) }}</p>
            </div>
            <div>
              <p class="text-xs font-medium uppercase text-gray-500">{{ t('admin.packages.fields.download') }}</p>
              <p class="mt-2 text-gray-800 dark:text-gray-200">
                {{ t('admin.packages.labels.downloadCount', { count: item.download_count }) }}<span v-if="item.last_downloaded_at"> · {{ formatDate(item.last_downloaded_at) }}</span>
              </p>
            </div>
            <div>
              <p class="text-xs font-medium uppercase text-gray-500">{{ t('admin.packages.fields.storage') }}</p>
              <p class="mt-2 break-all text-gray-800 dark:text-gray-200">{{ item.storage_key || item.file_url || '-' }}</p>
            </div>
            <div v-if="item.review_note" class="md:col-span-2">
              <p class="text-xs font-medium uppercase text-gray-500">{{ t('admin.packages.fields.reviewNote') }}</p>
              <p class="mt-2 text-gray-800 dark:text-gray-200">{{ item.review_note }}</p>
            </div>
            <div v-if="item.takedown_reason" class="md:col-span-2">
              <p class="text-xs font-medium uppercase text-gray-500">{{ t('admin.packages.fields.takedownReason') }}</p>
              <p class="mt-2 text-red-600 dark:text-red-400">{{ item.takedown_reason }}</p>
            </div>
          </div>

          <div class="flex flex-wrap justify-end gap-2 border-t border-gray-100 px-5 py-4 dark:border-dark-700">
            <button type="button" class="btn btn-secondary btn-sm" @click="recordDownload(item)">
              <Icon name="download" size="sm" />
              {{ t('admin.packages.actions.recordDownload') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" @click="openBindingDialog(item)">{{ t('admin.packages.actions.bindings') }}</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="item.status === 'approved'" @click="reviewPackage(item, 'approved')">{{ t('admin.packages.actions.approve') }}</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="item.status === 'rejected'" @click="reviewPackage(item, 'rejected')">{{ t('admin.packages.actions.reject') }}</button>
            <button type="button" class="btn btn-danger btn-sm" :disabled="item.status === 'taken_down'" @click="takeDownPackage(item)">{{ t('admin.packages.actions.takeDown') }}</button>
            <button type="button" class="btn btn-danger btn-sm" :title="t('admin.packages.actions.delete')" @click="deletePackage(item)">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </article>
      </div>

      <BaseDialog :show="showEditDialog" :title="editingPackage ? t('admin.packages.dialogs.edit') : t('admin.packages.dialogs.create')" width="wide" @close="closeEditDialog">
        <form id="package-form" class="space-y-4" @submit.prevent="savePackage">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.packages.fields.name') }}</label>
              <input v-model="form.name" class="input" required />
            </div>
            <div>
              <label class="input-label">{{ t('admin.packages.fields.version') }}</label>
              <input v-model="form.version" class="input" :placeholder="t('admin.packages.placeholders.version')" />
            </div>
          </div>
          <div>
            <label class="input-label">{{ t('admin.packages.fields.description') }}</label>
            <textarea v-model="form.description" class="input" rows="3"></textarea>
          </div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.packages.fields.status') }}</label>
              <select v-model="form.status" class="input">
                <option v-for="status in packageStatuses" :key="status" :value="status">{{ statusLabel(status) }}</option>
              </select>
            </div>
            <div>
              <label class="input-label">{{ t('admin.packages.fields.sizeBytes') }}</label>
              <input v-model.number="form.size_bytes" class="input" min="0" type="number" />
            </div>
          </div>
          <div>
            <label class="input-label">{{ t('admin.packages.fields.fileURL') }}</label>
            <input v-model="form.file_url" class="input" :placeholder="t('admin.packages.placeholders.fileURL')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.packages.fields.storageKey') }}</label>
            <input v-model="form.storage_key" class="input" :placeholder="t('admin.packages.placeholders.storageKey')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.packages.fields.checksum') }}</label>
            <input v-model="form.checksum" class="input" :placeholder="t('admin.packages.placeholders.checksum')" />
          </div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.packages.fields.planBindings') }}</label>
              <input v-model="planBindingsText" class="input" :placeholder="t('admin.packages.placeholders.commaPlans')" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.packages.fields.permissionBindings') }}</label>
              <input v-model="permissionBindingsText" class="input" :placeholder="t('admin.packages.placeholders.commaPermissions')" />
            </div>
          </div>
        </form>
        <template #footer>
          <div class="flex justify-end gap-3">
            <button type="button" class="btn btn-secondary" @click="closeEditDialog">{{ t('common.cancel') }}</button>
            <button type="submit" form="package-form" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button>
          </div>
        </template>
      </BaseDialog>

      <BaseDialog :show="bindingPackage !== null" :title="t('admin.packages.dialogs.bindings')" width="normal" @close="bindingPackage = null">
        <div class="space-y-4">
          <div>
            <label class="input-label">{{ t('admin.packages.fields.planBindings') }}</label>
            <input v-model="bindingPlanText" class="input" :placeholder="t('admin.packages.placeholders.commaSeparated')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.packages.fields.permissionBindings') }}</label>
            <input v-model="bindingPermissionText" class="input" :placeholder="t('admin.packages.placeholders.commaSeparated')" />
          </div>
        </div>
        <template #footer>
          <div class="flex justify-end gap-3">
            <button type="button" class="btn btn-secondary" @click="bindingPackage = null">{{ t('common.cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveBindings">{{ t('admin.packages.actions.saveBindings') }}</button>
          </div>
        </template>
      </BaseDialog>

      <BaseDialog :show="showStorageDialog" :title="t('admin.packages.dialogs.storage')" width="wide" @close="showStorageDialog = false">
        <form id="storage-form" class="space-y-4" @submit.prevent="saveStorage">
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.packages.fields.driver') }}</label>
              <select v-model="storageForm.driver" class="input">
                <option value="local">{{ t('admin.packages.storageDrivers.local') }}</option>
                <option value="s3">{{ t('admin.packages.storageDrivers.s3') }}</option>
              </select>
            </div>
            <div>
              <label class="input-label">{{ t('admin.packages.fields.publicBaseURL') }}</label>
              <input v-model="storageForm.public_base_url" class="input" :placeholder="t('admin.packages.placeholders.publicBaseURL')" />
            </div>
          </div>
          <div v-if="storageForm.driver === 'local'">
            <label class="input-label">{{ t('admin.packages.fields.localPath') }}</label>
            <input v-model="storageForm.base_path" class="input" />
          </div>
          <template v-else>
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="storageForm.reuse_backup_s3" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ t('admin.packages.fields.reuseBackupS3') }}
            </label>
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div>
                <label class="input-label">{{ t('admin.packages.fields.bucket') }}</label>
                <input v-model="storageForm.bucket" class="input" :placeholder="storageForm.reuse_backup_s3 ? t('admin.packages.placeholders.bucketInherited') : ''" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.packages.fields.prefix') }}</label>
                <input v-model="storageForm.prefix" class="input" :placeholder="t('admin.packages.placeholders.prefix')" />
              </div>
            </div>
            <template v-if="!storageForm.reuse_backup_s3">
              <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                <div>
                  <label class="input-label">{{ t('admin.packages.fields.endpoint') }}</label>
                  <input v-model="storageForm.endpoint" class="input" :placeholder="t('admin.packages.placeholders.endpoint')" />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.packages.fields.region') }}</label>
                  <input v-model="storageForm.region" class="input" :placeholder="t('admin.packages.placeholders.region')" />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.packages.fields.accessKeyID') }}</label>
                  <input v-model="storageForm.access_key_id" class="input" />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.packages.fields.secretAccessKey') }}</label>
                  <input v-model="storageForm.secret_access_key" type="password" class="input" :placeholder="storageForm.secret_access_key_configured ? t('admin.packages.placeholders.secretConfigured') : ''" />
                </div>
              </div>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                <input v-model="storageForm.force_path_style" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                {{ t('admin.packages.fields.forcePathStyle') }}
              </label>
            </template>
            <div>
              <label class="input-label">{{ t('admin.packages.fields.presignExpiryHours') }}</label>
              <input v-model.number="storageForm.presign_expiry_hours" type="number" min="1" class="input" />
            </div>
          </template>
        </form>
        <template #footer>
          <div class="flex justify-end gap-3">
            <button type="button" class="btn btn-secondary" @click="showStorageDialog = false">{{ t('common.cancel') }}</button>
            <button type="submit" form="storage-form" class="btn btn-primary">{{ t('admin.packages.actions.saveConfig') }}</button>
          </div>
        </template>
      </BaseDialog>

      <BaseDialog :show="auditPackage !== null" :title="t('admin.packages.dialogs.audits')" width="extra-wide" @close="auditPackage = null">
        <div class="space-y-3">
          <p class="text-sm text-gray-500">{{ auditPackage?.name || t('admin.packages.labels.allPackages') }}</p>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[720px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 text-left text-xs uppercase text-gray-500 dark:bg-dark-800">
                <tr>
                  <th class="px-3 py-2">{{ t('admin.packages.fields.time') }}</th>
                  <th class="px-3 py-2">{{ t('admin.packages.fields.package') }}</th>
                  <th class="px-3 py-2">{{ t('admin.packages.fields.user') }}</th>
                  <th class="px-3 py-2">{{ t('admin.packages.fields.ip') }}</th>
                  <th class="px-3 py-2">{{ t('admin.packages.fields.userAgent') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="audit in audits" :key="audit.id">
                  <td class="px-3 py-2">{{ formatDate(audit.created_at) }}</td>
                  <td class="px-3 py-2">{{ audit.package_name }}</td>
                  <td class="px-3 py-2">{{ audit.actor_user_id || '-' }}</td>
                  <td class="px-3 py-2">{{ audit.client_ip || '-' }}</td>
                  <td class="max-w-sm truncate px-3 py-2">{{ audit.user_agent || '-' }}</td>
                </tr>
                <tr v-if="audits.length === 0">
                  <td class="px-3 py-8 text-center text-gray-500" colspan="5">{{ t('admin.packages.labels.noAudits') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </BaseDialog>
    </div>
    <TotpStepUpDialog :controller="cloudKeyStepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AdminPackage, type AdminPackageStatus, type PackageCloudKeyStatus, type PackageDownloadAudit, type PackageStorageConfig, type SaveAdminPackageRequest } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode, extractApiErrorMetadata } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'

const { t, te } = useI18n()
const appStore = useAppStore()
const cloudKeyStepUp = useStepUp()
const packages = ref<AdminPackage[]>([])
const audits = ref<PackageDownloadAudit[]>([])
const cloudKeyStatus = ref<PackageCloudKeyStatus>({ configured: false, key_id: '', algorithm: '', source: '', public_key: '' })
const loading = ref(false)
const saving = ref(false)
const generatingCloudKey = ref(false)
const loadingCloudKey = ref(false)
const showEditDialog = ref(false)
const showStorageDialog = ref(false)
const editingPackage = ref<AdminPackage | null>(null)
const bindingPackage = ref<AdminPackage | null>(null)
const auditPackage = ref<AdminPackage | null>(null)
const planBindingsText = ref('')
const permissionBindingsText = ref('')
const bindingPlanText = ref('')
const bindingPermissionText = ref('')

const packageStatuses: AdminPackageStatus[] = ['draft', 'pending_review', 'approved', 'rejected', 'taken_down']

const filters = reactive({ search: '', status: '' })
const form = reactive<SaveAdminPackageRequest>({ name: '', status: 'draft', size_bytes: 0 })
const storageForm = reactive<PackageStorageConfig>({
  driver: 'local',
  reuse_backup_s3: false,
  base_path: '',
  public_base_url: '',
  endpoint: '',
  region: 'auto',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  secret_access_key_configured: false,
  prefix: '',
  presign_expiry_hours: 24,
  force_path_style: false
})

const totalDownloads = computed(() => packages.value.reduce((sum, item) => sum + item.download_count, 0))

function statusCount(status: AdminPackageStatus): number {
  return packages.value.filter((item) => item.status === status).length
}

function statusLabel(status: AdminPackageStatus): string {
  return t(`admin.packages.status.${status}`)
}

function statusClass(status: AdminPackageStatus): string {
  if (status === 'approved') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'pending_review') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (status === 'rejected' || status === 'taken_down') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

function listText(values?: string[]): string {
  return values?.length ? values.join(', ') : '-'
}

function splitList(value: string): string[] {
  return value.split(',').map((item) => item.trim()).filter(Boolean)
}

function formatDate(value?: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function sourceLabel(source: string): string {
  const key = `admin.packages.cloudKey.sources.${source || 'none'}`
  return te(key) ? t(key) : source || '-'
}

function packageErrorMessage(error: unknown, fallbackKey = 'admin.packages.errors.fallback'): string {
  const code = extractApiErrorCode(error)
  if (code) {
    const key = `admin.packages.errors.${code}`
    if (te(key)) {
      return t(key, extractApiErrorMetadata(error) ?? {})
    }
  }
  return t(fallbackKey)
}

async function loadPackages(): Promise<void> {
  loading.value = true
  try {
    packages.value = await adminAPI.packages.list({
      status: filters.status || undefined,
      search: filters.search || undefined
    })
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadStorage(): Promise<void> {
  try {
    Object.assign(storageForm, await adminAPI.packages.getStorage())
    storageForm.secret_access_key = ''
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.storageLoadFailed'))
  }
}

async function loadCloudKey(): Promise<void> {
  if (loadingCloudKey.value || generatingCloudKey.value) return
  loadingCloudKey.value = true
  try {
    cloudKeyStatus.value = await adminAPI.packages.getCloudKey()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.cloudKeyLoadFailed'))
  } finally {
    loadingCloudKey.value = false
  }
}

async function loadAll(): Promise<void> {
  await Promise.all([loadPackages(), loadStorage(), loadCloudKey()])
}

function resetForm(): void {
  Object.assign(form, {
    name: '',
    description: '',
    version: '',
    status: 'draft',
    file_url: '',
    storage_key: '',
    size_bytes: 0,
    checksum: '',
    plan_bindings: [],
    permission_bindings: []
  })
  planBindingsText.value = ''
  permissionBindingsText.value = ''
}

function openCreateDialog(): void {
  editingPackage.value = null
  resetForm()
  showEditDialog.value = true
}

function openEditDialog(item: AdminPackage): void {
  editingPackage.value = item
  Object.assign(form, {
    name: item.name,
    description: item.description,
    version: item.version,
    status: item.status,
    file_url: item.file_url,
    storage_key: item.storage_key,
    size_bytes: item.size_bytes,
    checksum: item.checksum,
    plan_bindings: item.plan_bindings,
    permission_bindings: item.permission_bindings
  })
  planBindingsText.value = item.plan_bindings.join(', ')
  permissionBindingsText.value = item.permission_bindings.join(', ')
  showEditDialog.value = true
}

function closeEditDialog(): void {
  showEditDialog.value = false
  editingPackage.value = null
}

async function savePackage(): Promise<void> {
  saving.value = true
  try {
    const payload = {
      ...form,
      plan_bindings: splitList(planBindingsText.value),
      permission_bindings: splitList(permissionBindingsText.value)
    }
    if (editingPackage.value) {
      await adminAPI.packages.update(editingPackage.value.id, payload)
    } else {
      await adminAPI.packages.create(payload)
    }
    appStore.showSuccess(t('admin.packages.messages.saved'))
    closeEditDialog()
    await loadPackages()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openBindingDialog(item: AdminPackage): void {
  bindingPackage.value = item
  bindingPlanText.value = item.plan_bindings.join(', ')
  bindingPermissionText.value = item.permission_bindings.join(', ')
}

async function saveBindings(): Promise<void> {
  if (!bindingPackage.value) return
  try {
    await adminAPI.packages.updateBindings(bindingPackage.value.id, {
      plan_bindings: splitList(bindingPlanText.value),
      permission_bindings: splitList(bindingPermissionText.value)
    })
    appStore.showSuccess(t('admin.packages.messages.bindingsSaved'))
    bindingPackage.value = null
    await loadPackages()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.bindingsSaveFailed'))
  }
}

async function reviewPackage(item: AdminPackage, status: 'approved' | 'rejected'): Promise<void> {
  const promptKey = status === 'approved' ? 'admin.packages.prompts.approveNote' : 'admin.packages.prompts.rejectNote'
  const note = window.prompt(t(promptKey)) || ''
  try {
    await adminAPI.packages.review(item.id, status, note)
    appStore.showSuccess(t(status === 'approved' ? 'admin.packages.messages.approved' : 'admin.packages.messages.rejected'))
    await loadPackages()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.reviewFailed'))
  }
}

async function takeDownPackage(item: AdminPackage): Promise<void> {
  const reason = window.prompt(t('admin.packages.prompts.takedownReason')) || ''
  try {
    await adminAPI.packages.takeDown(item.id, reason)
    appStore.showSuccess(t('admin.packages.messages.takenDown'))
    await loadPackages()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.takedownFailed'))
  }
}

async function deletePackage(item: AdminPackage): Promise<void> {
  if (!window.confirm(t('admin.packages.confirms.delete', { name: item.name }))) return
  try {
    await adminAPI.packages.remove(item.id)
    appStore.showSuccess(t('admin.packages.messages.deleted'))
    await loadPackages()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.deleteFailed'))
  }
}

async function recordDownload(item: AdminPackage): Promise<void> {
  try {
    await adminAPI.packages.recordDownloadAudit(item.id)
    appStore.showSuccess(t('admin.packages.messages.auditRecorded'))
    await loadPackages()
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.auditRecordFailed'))
  }
}

async function openAuditDialog(item: AdminPackage): Promise<void> {
  auditPackage.value = item
  try {
    audits.value = await adminAPI.packages.listDownloadAudits({ package_id: item.id, limit: 100 })
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.auditLoadFailed'))
  }
}

async function openStorageDialog(): Promise<void> {
  await loadStorage()
  showStorageDialog.value = true
}

async function saveStorage(): Promise<void> {
  try {
    Object.assign(storageForm, await adminAPI.packages.updateStorage({ ...storageForm }))
    storageForm.secret_access_key = ''
    appStore.showSuccess(t('admin.packages.messages.storageSaved'))
    showStorageDialog.value = false
  } catch (error) {
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.storageSaveFailed'))
  }
}

async function generateCloudKey(): Promise<void> {
  if (cloudKeyStatus.value.configured || loadingCloudKey.value || generatingCloudKey.value) return
  generatingCloudKey.value = true
  try {
    cloudKeyStatus.value = await cloudKeyStepUp.run(() => adminAPI.packages.generateCloudKey())
    appStore.showSuccess(t('admin.packages.messages.cloudKeyGenerated'))
  } catch (error) {
    if (isStepUpCancelled(error)) return
    if (isStepUpBlocked(error)) {
      appStore.showError(t(stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? 'stepUp.adminApiKeyForbidden' : 'stepUp.notEnabled'))
      return
    }
    appStore.showError(packageErrorMessage(error, 'admin.packages.errors.cloudKeyGenerateFailed'))
  } finally {
    generatingCloudKey.value = false
  }
}

onMounted(loadAll)
</script>
