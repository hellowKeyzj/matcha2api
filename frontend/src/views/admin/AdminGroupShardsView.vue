<template>
  <AppLayout>
    <div class="space-y-6" data-testid="group-shards-page">
      <section>
        <div class="space-y-4">
          <div class="flex flex-col justify-between gap-4 lg:flex-row lg:items-start">
            <div class="min-w-0 space-y-2">
              <button
                type="button"
                class="inline-flex items-center gap-2 text-sm font-medium text-gray-500 transition-colors hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400"
                @click="goBack"
              >
                <Icon name="arrowLeft" size="sm" />
                {{ t("admin.groups.shards.backToGroups") }}
              </button>
              <div>
                <div class="flex flex-wrap items-center gap-2">
                  <h1 class="truncate text-2xl font-semibold text-gray-900 dark:text-white">
                    {{ t("admin.groups.shards.title") }}
                  </h1>
                  <span
                    v-if="group"
                    class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200"
                  >
                    <PlatformIcon :platform="group.platform" size="xs" />
                    {{ platformLabel(group.platform) }}
                  </span>
                </div>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ group ? t("admin.groups.shards.groupSubtitle", { name: group.name }) : t("admin.groups.shards.description") }}
                </p>
              </div>
            </div>

            <div class="flex flex-wrap items-center justify-end gap-3">
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="loading"
                @click="loadPage"
              >
                <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
                <span class="ml-2">{{ t("common.refresh") }}</span>
              </button>
              <button type="button" class="btn btn-primary" @click="openCreateShardDialog">
                <Icon name="plus" size="md" class="mr-2" />
                {{ t("admin.groups.shards.create") }}
              </button>
            </div>
          </div>

          <div
            v-if="group && !group.shard_enabled"
            class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200"
          >
            {{ t("admin.groups.shards.disabledHint") }}
          </div>
        </div>
      </section>

      <section>
        <div class="grid grid-cols-1 items-start gap-5 lg:grid-cols-[260px_minmax(0,1fr)]">
          <section class="rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
            <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-dark-700">
              <div>
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t("admin.groups.shards.listTitle") }}
                </h2>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t("admin.groups.shards.maxUsersSummary", { count: maxUsersPerShard }) }}
                </p>
              </div>
              <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                {{ shards.length }}
              </span>
            </div>

            <div class="border-b border-gray-200 p-3 dark:border-dark-700">
              <button type="button" class="flex w-full items-center justify-between rounded-lg px-3 py-3 text-left text-sm transition-colors"
                :class="poolView === 'shared' || !selectedShard ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-800'"
                :aria-pressed="poolView === 'shared' || !selectedShard"
                @click="poolView = 'shared'">
                <span class="font-semibold">{{ t('admin.groups.shards.sharedNav') }}</span>
                <span class="rounded-md bg-white px-2 py-0.5 text-xs tabular-nums dark:bg-dark-800">{{ sharedAccountDrafts.length }}</span>
              </button>
            </div>
            <div v-if="loading" class="space-y-2 p-4">
              <div v-for="i in 4" :key="i" class="h-20 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800" />
            </div>

            <div v-else-if="shards.length === 0" class="p-6">
              <EmptyState
                :title="t('admin.groups.shards.emptyTitle')"
                :description="t('admin.groups.shards.emptyDescription')"
                :action-text="t('admin.groups.shards.create')"
                @action="openCreateShardDialog"
              />
            </div>

            <div v-else class="max-h-[calc(100vh-18rem)] overflow-y-auto p-3">
              <button
                v-for="shard in shards"
                :key="shard.id"
                type="button"
                class="mb-2 w-full rounded-lg border px-3 py-3 text-left transition-colors"
                :class="poolView === 'shard' && selectedShardId === shard.id
                  ? 'border-primary-300 bg-primary-50 dark:border-primary-800 dark:bg-primary-900/20'
                  : 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600 dark:hover:bg-dark-800'"
                @click="selectShard(shard.id)"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="flex items-center gap-2">
                      <span class="truncate font-medium text-gray-900 dark:text-white">{{ shard.name }}</span>
                      <span :class="statusBadgeClass(shard.status)">
                        {{ shardStatusLabel(shard.status) }}
                      </span>
                    </div>
                    <p class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
                      {{ shard.description || t("admin.groups.shards.noDescription") }}
                    </p>
                  </div>
                  <span class="shrink-0 font-mono text-xs text-gray-400">#{{ shard.id }}</span>
                </div>
                <div class="mt-2 flex gap-4 text-xs">
                  <div class="py-1">
                    <span class="text-gray-500 dark:text-gray-400">{{ t("admin.groups.shards.users") }}</span>
                    <span class="ml-1 font-semibold text-gray-900 dark:text-white">{{ shard.user_count }}/{{ maxUsersPerShard }}</span>
                  </div>
                  <div class="py-1">
                    <span class="text-gray-500 dark:text-gray-400">{{ t("admin.groups.shards.accounts") }}</span>
                    <span class="ml-1 font-semibold text-gray-900 dark:text-white">{{ shard.account_count }}</span>
                  </div>
                </div>
              </button>
            </div>
          </section>

          <section class="min-w-0 rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
            <div v-show="poolView === 'shared' || !selectedShard" data-testid="shared-pool-panel">
            <GroupShardAccountPoolEditor
              v-if="group"
              v-model="sharedAccountDrafts"
              class="!rounded-xl !border-0"
              :group-id="groupID"
              :platform="group.platform"
              :excluded-account-ids="exclusiveAccountIDs"
              :saving="savingSharedAccounts"
              :disabled="loading || detailsLoading"
              title-key="admin.groups.shards.sharedAccountSectionTitle"
              hint-key="admin.groups.shards.sharedAccountSectionHint"
              @save="saveSharedAccounts"
            />
            </div>

            <div v-if="!selectedShard && !group" class="p-10">
              <EmptyState
                :title="t('admin.groups.shards.selectTitle')"
                :description="t('admin.groups.shards.selectDescription')"
              />
            </div>

            <div v-if="selectedShard" v-show="poolView === 'shard'">
              <div class="flex flex-col justify-between gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-start">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <h2 class="truncate text-lg font-semibold text-gray-900 dark:text-white">
                      {{ selectedShard.name }}
                    </h2>
                    <span :class="statusBadgeClass(selectedShard.status)">
                      {{ shardStatusLabel(selectedShard.status) }}
                    </span>
                  </div>
                  <p v-if="selectedShard.description" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                    {{ selectedShard.description }}
                  </p>
                  <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    {{ t("admin.groups.shards.updatedAt", { time: formatDateTime(selectedShard.updated_at) }) }}
                  </p>
                </div>
                <div class="flex flex-wrap gap-2">
                  <button type="button" class="btn btn-secondary" @click="openEditShardDialog(selectedShard)">
                    <Icon name="edit" size="sm" class="mr-2" />
                    {{ t("common.edit") }}
                  </button>
                  <button type="button" class="btn btn-danger" @click="confirmDeleteShard(selectedShard)">
                    <Icon name="trash" size="sm" class="mr-2" />
                    {{ t("common.delete") }}
                  </button>
                </div>
              </div>

              <div v-if="detailsLoading" class="space-y-4 p-5">
                <div v-for="i in 3" :key="i" class="h-28 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-800" />
              </div>

              <div v-else class="space-y-5 p-4 sm:p-5">
                <section class="border-b border-gray-200 pb-4 dark:border-dark-700">
                  <div class="flex flex-col justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center">
                    <div>
                      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                        {{ t("admin.groups.shards.userSectionTitle") }}
                      </h3>
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t("admin.groups.shards.userCapacity", { used: assignedUsers.length, max: maxUsersPerShard }) }}
                      </p>
                    </div>
                    <div class="relative w-full lg:w-80">
                      <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                      <input
                        v-model="userSearchQuery"
                        type="text"
                        autocomplete="off"
                        class="input input-sm w-full pl-9"
                        :disabled="userCapacityFull || addingUser"
                        :placeholder="userCapacityFull ? t('admin.groups.shards.userCapacityFull') : t('admin.groups.shards.searchUsers')"
                        @input="debounceUserSearch"
                        @focus="userSearchQuery.trim() && loadUserCandidates()"
                      />
                      <div
                        v-if="userSearchQuery.trim()"
                        class="absolute z-40 mt-1 max-h-64 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-800"
                      >
                        <div v-if="userSearchLoading" class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                          {{ t("common.loading") }}
                        </div>
                        <div v-else-if="availableUserResults.length === 0" class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                          {{ t("admin.groups.shards.noUserResults") }}
                        </div>
                        <template v-else>
                          <button
                            v-for="user in availableUserResults"
                            :key="user.id"
                            type="button"
                            class="flex w-full items-center justify-between gap-3 px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-700"
                            :disabled="addingUser || userCapacityFull"
                            @click="assignUser(user)"
                          >
                            <span class="min-w-0 truncate">
                              <span class="font-medium text-gray-900 dark:text-white">{{ user.email }}</span>
                              <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">{{ user.username || '-' }}</span>
                            </span>
                            <span class="shrink-0 font-mono text-xs text-gray-400">#{{ user.id }}</span>
                          </button>
                        </template>
                      </div>
                    </div>
                  </div>

                  <div class="divide-y divide-gray-200 dark:divide-dark-700">
                    <div v-if="assignedUsers.length === 0" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                      {{ t("admin.groups.shards.noAssignedUsers") }}
                    </div>
                    <div
                      v-for="user in assignedUsers"
                      :key="user.user_id"
                      class="flex flex-col justify-between gap-2 px-4 py-2 sm:flex-row sm:items-center"
                    >
                      <div class="min-w-0">
                        <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
                          {{ user.email }}
                        </div>
                        <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                          <span>{{ user.username || '-' }}</span>
                          <span class="font-mono">#{{ user.user_id }}</span>
                          <span>{{ formatDateTime(user.created_at) }}</span>
                        </div>
                      </div>
                      <button
                        type="button"
                        class="btn btn-sm btn-secondary text-red-600 hover:border-red-200 hover:bg-red-50 hover:text-red-700 dark:text-red-400 dark:hover:bg-red-900/20"
                        :disabled="removingUserIds.has(user.user_id)"
                        @click="removeUser(user)"
                      >
                        <Icon name="x" size="xs" class="mr-1" />
                        {{ t("common.remove") }}
                      </button>
                    </div>
                  </div>
                </section>

                <GroupShardAccountPoolEditor
                  v-if="group"
                  :key="selectedShard.id"
                  v-model="accountDrafts"
                  :group-id="groupID"
                  :platform="group.platform"
                  :excluded-account-ids="sharedAccountIDs"
                  :saving="savingAccounts"
                  title-key="admin.groups.shards.accountSectionTitle"
                  hint-key="admin.groups.shards.accountSectionHint"
                  @save="saveAccounts"
                />
              </div>
            </div>
          </section>
        </div>
      </section>
    </div>

    <BaseDialog
      :show="showShardDialog"
      :title="editingShard ? t('admin.groups.shards.edit') : t('admin.groups.shards.create')"
      width="normal"
      @close="closeShardDialog"
    >
      <form id="group-shard-form" class="space-y-4" @submit.prevent="saveShard">
        <div>
          <label class="input-label">
            {{ t("admin.groups.shards.name") }}
            <span class="text-red-500">*</span>
          </label>
          <input v-model="shardForm.name" type="text" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t("admin.groups.shards.formDescription") }}</label>
          <textarea v-model="shardForm.description" rows="3" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t("admin.groups.shards.status") }}</label>
          <select v-model="shardForm.status" class="input">
            <option value="active">{{ t("admin.groups.shards.statusActive") }}</option>
            <option value="inactive">{{ t("admin.groups.shards.statusInactive") }}</option>
          </select>
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeShardDialog">
          {{ t("common.cancel") }}
        </button>
        <button type="submit" form="group-shard-form" class="btn btn-primary" :disabled="savingShard">
          {{ savingShard ? t("admin.groups.shards.saving") : t("common.save") }}
        </button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.groups.shards.deleteTitle')"
      :message="deleteTarget ? t('admin.groups.shards.deleteConfirm', { name: deleteTarget.name }) : ''"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="deleteShard"
      @cancel="closeDeleteDialog"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { adminAPI } from "@/api/admin";
import type {
  AccountPlatform,
  AccountType,
  AdminGroup,
  AdminUser,
  GroupPlatform,
  GroupShard,
  GroupShardAccount,
  GroupShardUser,
} from "@/types";
import { CONCRETE_PLATFORM_OPTIONS } from "@/constants/platforms";
import { useAppStore } from "@/stores/app";
import { extractApiErrorMessage } from "@/utils/apiError";
import AppLayout from "@/components/layout/AppLayout.vue";
import BaseDialog from "@/components/common/BaseDialog.vue";
import ConfirmDialog from "@/components/common/ConfirmDialog.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import PlatformIcon from "@/components/common/PlatformIcon.vue";
import GroupShardAccountPoolEditor from "./GroupShardAccountPoolEditor.vue";
import Icon from "@/components/icons/Icon.vue";

interface AccountDraft {
  account_id: number;
  name: string;
  platform: AccountPlatform;
  type: AccountType | string;
  status: string;
  priority: number;
}

interface ShardForm {
  name: string;
  description: string;
  status: "active" | "inactive";
}

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const appStore = useAppStore();

const group = ref<AdminGroup | null>(null);
const shards = ref<GroupShard[]>([]);
const selectedShardId = ref<number | null>(null);
const poolView = ref<"shared" | "shard">("shard");
const assignedUsers = ref<GroupShardUser[]>([]);
const accountDrafts = ref<AccountDraft[]>([]);
const sharedAccountDrafts = ref<AccountDraft[]>([]);
const savedSharedAccountIDs = ref<number[]>([]);
const savedExclusiveAccountIDs = ref<number[]>([]);
const savingSharedAccounts = ref(false);
const sharedAccountIDs = computed(() => [...savedSharedAccountIDs.value, ...sharedAccountDrafts.value.map((account) => account.account_id)]);
const exclusiveAccountIDs = computed(() => [...savedExclusiveAccountIDs.value, ...accountDrafts.value.map((account) => account.account_id)]);
const loading = ref(false);
const detailsLoading = ref(false);
const savingShard = ref(false);
const savingAccounts = ref(false);
const addingUser = ref(false);
const userSearchQuery = ref("");
const userSearchResults = ref<AdminUser[]>([]);
const userSearchLoading = ref(false);
const showShardDialog = ref(false);
const editingShard = ref<GroupShard | null>(null);
const showDeleteDialog = ref(false);
const deleteTarget = ref<GroupShard | null>(null);
const removingUserIds = reactive(new Set<number>());
const shardForm = reactive<ShardForm>({
  name: "",
  description: "",
  status: "active",
});
let userSearchTimer: ReturnType<typeof setTimeout> | null = null;
let userSearchSequence = 0;
let userSearchAbort: AbortController | null = null;
let detailRequestSequence = 0;
let detailRequestAbort: AbortController | null = null;

const groupID = computed(() => {
  const raw = Array.isArray(route.params.id) ? route.params.id[0] : route.params.id;
  const id = Number(raw);
  return Number.isSafeInteger(id) && id > 0 ? id : 0;
});

const selectedShard = computed(() => shards.value.find((shard) => shard.id === selectedShardId.value) ?? null);
const maxUsersPerShard = computed(() => Math.max(1, Number(group.value?.shard_max_users_per_shard) || 2));
const userCapacityFull = computed(() => assignedUsers.value.length >= maxUsersPerShard.value);

const providerSections = computed(() => {
  if (!group.value) return [];
  if (group.value.platform === "composite") return [...CONCRETE_PLATFORM_OPTIONS];
  return CONCRETE_PLATFORM_OPTIONS.filter((provider) => provider.value === group.value?.platform);
});

const assignedUserIDs = computed(() => new Set(assignedUsers.value.map((user) => user.user_id)));
const availableUserResults = computed(() =>
  userSearchResults.value.filter((user) => !assignedUserIDs.value.has(user.id)),
);

function goBack(): void {
  router.push({ name: "AdminGroups" });
}

function platformLabel(platform: GroupPlatform | AccountPlatform): string {
  return t(`admin.groups.platforms.${platform}`, platform);
}

function shardStatusLabel(status: string): string {
  return status === "inactive" ? t("admin.groups.shards.statusInactive") : t("admin.groups.shards.statusActive");
}

function statusBadgeClass(status: string): string {
  if (status === "inactive") {
    return "inline-flex shrink-0 items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300";
  }
  return "inline-flex shrink-0 items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-300";
}

function formatDateTime(value?: string | null): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function sortShards(list: GroupShard[]): GroupShard[] {
  return [...list].sort((left, right) => left.id - right.id);
}

function sortUsers(list: GroupShardUser[]): GroupShardUser[] {
  return [...list].sort((left, right) => left.user_id - right.user_id);
}

function sortAccountDrafts(list: AccountDraft[]): AccountDraft[] {
  const order = new Map(providerSections.value.map((provider, index) => [provider.value, index]));
  return [...list].sort((left, right) => {
    const platformOrder = (order.get(left.platform) ?? 99) - (order.get(right.platform) ?? 99);
    if (platformOrder !== 0) return platformOrder;
    if (left.priority !== right.priority) return left.priority - right.priority;
    return left.account_id - right.account_id;
  });
}

function accountDraftFromShardAccount(account: GroupShardAccount): AccountDraft {
  return {
    account_id: account.account_id,
    name: account.name,
    platform: account.platform,
    type: account.type,
    status: account.status,
    priority: normalizePriority(account.priority),
  };
}

function normalizePriority(value: unknown): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? Math.trunc(parsed) : 100;
}

function clearSearchState(): void {
  userSearchQuery.value = "";
  userSearchResults.value = [];
  userSearchLoading.value = false;
  userSearchAbort?.abort();
  userSearchAbort = null;
  userSearchSequence += 1;
  if (userSearchTimer) {
    clearTimeout(userSearchTimer);
    userSearchTimer = null;
  }
}

function cancelDetailRequest(): void {
  detailRequestAbort?.abort();
  detailRequestAbort = null;
  detailRequestSequence += 1;
  detailsLoading.value = false;
}

async function loadPage(): Promise<void> {
  if (groupID.value <= 0) {
    appStore.showError(t("admin.groups.shards.invalidGroup"));
    return;
  }
  loading.value = true;
  try {
    const [loadedGroup, loadedShards, sharedAccounts] = await Promise.all([
      adminAPI.groups.getById(groupID.value),
      adminAPI.groups.listShards(groupID.value),
      adminAPI.groups.listSharedAccounts(groupID.value),
    ]);
    group.value = loadedGroup;
    shards.value = sortShards(loadedShards);
    sharedAccountDrafts.value = sortAccountDrafts(sharedAccounts.map(accountDraftFromShardAccount));
    savedSharedAccountIDs.value = sharedAccounts.map((account) => account.account_id);
    if (!selectedShardId.value || !shards.value.some((shard) => shard.id === selectedShardId.value)) {
      selectedShardId.value = shards.value[0]?.id ?? null;
    }
    await loadSelectedShardDetails();
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("admin.groups.shards.loadFailed")));
  } finally {
    loading.value = false;
  }
}

async function refreshShards(): Promise<void> {
  const loadedShards = await adminAPI.groups.listShards(groupID.value);
  shards.value = sortShards(loadedShards);
  if (selectedShardId.value && !shards.value.some((shard) => shard.id === selectedShardId.value)) {
    selectedShardId.value = shards.value[0]?.id ?? null;
  }
}

async function loadSelectedShardDetails(): Promise<void> {
  if (!selectedShard.value || groupID.value <= 0) {
    cancelDetailRequest();
    assignedUsers.value = [];
    accountDrafts.value = [];
    savedExclusiveAccountIDs.value = [];
    return;
  }

  detailRequestAbort?.abort();
  const controller = new AbortController();
  detailRequestAbort = controller;
  const sequence = detailRequestSequence + 1;
  detailRequestSequence = sequence;
  const requestGroupID = groupID.value;
  const shardID = selectedShard.value.id;
  detailsLoading.value = true;

  try {
    const [users, accounts] = await Promise.all([
      adminAPI.groups.listShardUsers(requestGroupID, shardID, { signal: controller.signal }),
      adminAPI.groups.listShardAccounts(requestGroupID, shardID, { signal: controller.signal }),
    ]);
    if (sequence !== detailRequestSequence || selectedShardId.value !== shardID || groupID.value !== requestGroupID) {
      return;
    }
    assignedUsers.value = sortUsers(users);
    accountDrafts.value = sortAccountDrafts(accounts.map(accountDraftFromShardAccount));
    savedExclusiveAccountIDs.value = accounts.map((account) => account.account_id);
  } catch (error: unknown) {
    if (controller.signal.aborted || sequence !== detailRequestSequence) return;
    appStore.showError(extractApiErrorMessage(error, t("admin.groups.shards.loadDetailsFailed")));
  } finally {
    if (sequence === detailRequestSequence) {
      detailsLoading.value = false;
      if (detailRequestAbort === controller) detailRequestAbort = null;
    }
  }
}

function selectShard(shardID: number): void {
  poolView.value = "shard";
  if (selectedShardId.value === shardID) return;
  selectedShardId.value = shardID;
  clearSearchState();
  void loadSelectedShardDetails();
}

function openCreateShardDialog(): void {
  editingShard.value = null;
  shardForm.name = "";
  shardForm.description = "";
  shardForm.status = "active";
  showShardDialog.value = true;
}

function openEditShardDialog(shard: GroupShard): void {
  editingShard.value = shard;
  shardForm.name = shard.name;
  shardForm.description = shard.description || "";
  shardForm.status = shard.status;
  showShardDialog.value = true;
}

function closeShardDialog(): void {
  if (savingShard.value) return;
  showShardDialog.value = false;
  editingShard.value = null;
}

async function saveShard(): Promise<void> {
  if (groupID.value <= 0) return;
  const name = shardForm.name.trim();
  if (!name) {
    appStore.showError(t("admin.groups.shards.nameRequired"));
    return;
  }
  savingShard.value = true;
  try {
    const payload = {
      name,
      description: shardForm.description.trim(),
      status: shardForm.status,
    };
    const saved = editingShard.value
      ? await adminAPI.groups.updateShard(groupID.value, editingShard.value.id, payload)
      : await adminAPI.groups.createShard(groupID.value, payload);
    appStore.showSuccess(editingShard.value ? t("admin.groups.shards.updated") : t("admin.groups.shards.created"));
    showShardDialog.value = false;
    editingShard.value = null;
    await refreshShards();
    selectedShardId.value = saved.id;
    poolView.value = "shard";
    await loadSelectedShardDetails();
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("admin.groups.shards.saveFailed")));
  } finally {
    savingShard.value = false;
  }
}

function confirmDeleteShard(shard: GroupShard): void {
  deleteTarget.value = shard;
  showDeleteDialog.value = true;
}

function closeDeleteDialog(): void {
  showDeleteDialog.value = false;
  deleteTarget.value = null;
}

async function deleteShard(): Promise<void> {
  if (!deleteTarget.value || groupID.value <= 0) return;
  const deletingID = deleteTarget.value.id;
  try {
    await adminAPI.groups.deleteShard(groupID.value, deletingID);
    appStore.showSuccess(t("admin.groups.shards.deleted"));
    closeDeleteDialog();
    await refreshShards();
    if (selectedShardId.value === deletingID) selectedShardId.value = shards.value[0]?.id ?? null;
    await loadSelectedShardDetails();
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("admin.groups.shards.deleteFailed")));
  }
}

function debounceUserSearch(): void {
  if (userSearchTimer) clearTimeout(userSearchTimer);
  const query = userSearchQuery.value.trim();
  userSearchSequence += 1;
  const sequence = userSearchSequence;
  if (!query) {
    userSearchResults.value = [];
    userSearchLoading.value = false;
    return;
  }
  userSearchTimer = setTimeout(() => {
    void loadUserCandidates(sequence);
  }, 300);
}

async function loadUserCandidates(sequence = ++userSearchSequence): Promise<void> {
  const query = userSearchQuery.value.trim();
  if (!query) {
    userSearchResults.value = [];
    return;
  }
  userSearchAbort?.abort();
  const controller = new AbortController();
  userSearchAbort = controller;
  userSearchLoading.value = true;
  try {
    const result = await adminAPI.users.list(1, 20, { search: query }, { signal: controller.signal });
    if (sequence === userSearchSequence) userSearchResults.value = result.items || [];
  } catch (error: unknown) {
    if (!controller.signal.aborted) userSearchResults.value = [];
  } finally {
    if (sequence === userSearchSequence) userSearchLoading.value = false;
  }
}

async function assignUser(user: AdminUser): Promise<void> {
  if (!selectedShard.value || userCapacityFull.value || groupID.value <= 0) return;
  addingUser.value = true;
  try {
    await adminAPI.groups.assignShardUser(groupID.value, selectedShard.value.id, user.id);
    appStore.showSuccess(t("admin.groups.shards.userAssigned"));
    clearSearchState();
    await Promise.all([loadSelectedShardDetails(), refreshShards()]);
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("admin.groups.shards.userAssignFailed")));
  } finally {
    addingUser.value = false;
  }
}

async function removeUser(user: GroupShardUser): Promise<void> {
  if (!selectedShard.value || groupID.value <= 0 || removingUserIds.has(user.user_id)) return;
  removingUserIds.add(user.user_id);
  try {
    await adminAPI.groups.removeShardUser(groupID.value, selectedShard.value.id, user.user_id);
    appStore.showSuccess(t("admin.groups.shards.userRemoved"));
    await refreshShards();
    await loadSelectedShardDetails();
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("admin.groups.shards.userRemoveFailed")));
  } finally {
    removingUserIds.delete(user.user_id);
  }
}

async function saveSharedAccounts(): Promise<void> {
  if (groupID.value <= 0) return;
  savingSharedAccounts.value = true;
  try {
    const saved = await adminAPI.groups.setSharedAccounts(groupID.value, sharedAccountDrafts.value.map((account) => ({
      account_id: account.account_id,
      priority: normalizePriority(account.priority),
    })));
    sharedAccountDrafts.value = sortAccountDrafts(saved.map(accountDraftFromShardAccount));
    savedSharedAccountIDs.value = saved.map((account) => account.account_id);
    appStore.showSuccess(t("admin.groups.shards.accountsSaved"));
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("admin.groups.shards.accountsSaveFailed")));
  } finally {
    savingSharedAccounts.value = false;
  }
}

async function saveAccounts(): Promise<void> {
  if (!selectedShard.value || groupID.value <= 0) return;
  const requestGroupID = groupID.value;
  const shardID = selectedShard.value.id;
  savingAccounts.value = true;
  try {
    const accounts = accountDrafts.value.map((account) => ({
      account_id: account.account_id,
      priority: normalizePriority(account.priority),
    }));
    const saved = await adminAPI.groups.setShardAccounts(requestGroupID, shardID, accounts);
    if (groupID.value !== requestGroupID) return;
    if (selectedShardId.value === shardID) {
      accountDrafts.value = sortAccountDrafts(saved.map(accountDraftFromShardAccount));
      savedExclusiveAccountIDs.value = saved.map((account) => account.account_id);
    }
    appStore.showSuccess(t("admin.groups.shards.accountsSaved"));
    await refreshShards();
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t("admin.groups.shards.accountsSaveFailed")));
  } finally {
    savingAccounts.value = false;
  }
}

watch(groupID, () => {
  selectedShardId.value = null;
  cancelDetailRequest();
  clearSearchState();
  void loadPage();
});

onMounted(() => {
  void loadPage();
});

onUnmounted(() => {
  cancelDetailRequest();
  clearSearchState();
});
</script>
