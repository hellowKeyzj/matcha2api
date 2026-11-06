<template>
  <section class="min-w-0 rounded-xl border border-gray-200 bg-surface dark:border-dark-700 dark:bg-dark-900">
    <div class="flex flex-col justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700 lg:flex-row lg:items-center">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t(titleKey) }}
        </h3>
        <p class="mt-1 max-w-3xl text-xs leading-relaxed text-gray-500 dark:text-gray-400">
          {{ t(hintKey) }}
        </p>
      </div>
      <button
        type="button"
        class="btn btn-sm btn-primary shrink-0"
        :disabled="saving || disabled"
        @click="$emit('save')"
      >
        <Icon name="check" size="sm" class="mr-2" />
        {{ saving ? t("admin.groups.shards.savingAccounts") : t("admin.groups.shards.saveAccounts") }}
      </button>
    </div>

    <fieldset :disabled="saving || disabled" class="space-y-5 p-4">
      <div
        v-for="provider in providerSections"
        :key="provider.value"
        class="min-w-0"
      >
        <div class="flex flex-col justify-between gap-3 lg:flex-row lg:items-center">
          <div class="flex items-center gap-2">
            <PlatformIcon :platform="provider.value" size="sm" />
            <div>
              <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ provider.label }}
              </h4>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t("admin.groups.shards.providerAccountCount", { count: selectedAccountsByProvider(provider.value).length }) }}
              </p>
            </div>
          </div>
          <Select
            class="w-full lg:w-80"
            :model-value="null"
            :options="availableAccountResults(provider.value).map(account => ({ ...account, value: account.id, label: account.name }))"
            :placeholder="t('admin.groups.shards.searchAccounts')"
            :aria-label="t('admin.groups.shards.searchAccounts')"
            :search-placeholder="t('admin.groups.shards.searchAccounts')"
            :empty-text="t('admin.groups.shards.noAccountResults')"
            :disabled="saving || disabled"
            :loading="accountSearchLoading[provider.value]"
            remote
            load-on-open
            multiple
            :selected-values="pendingAccounts(provider.value).map(account => account.id)"
            @search="loadAccountCandidates(provider.value, $event)"
            @toggle="toggleAccount(provider.value, Number($event.value))"
          >
            <template #option="{ option, selected }">
              <span aria-hidden="true" class="mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded border" :class="selected ? 'border-primary-500 bg-primary-500 text-white' : 'border-gray-300 dark:border-dark-500'">
                <Icon v-if="selected" name="check" size="sm" />
              </span>
              <span class="min-w-0 flex-1 truncate">{{ option.label }}</span>
              <span class="ml-2 shrink-0 text-xs text-gray-400">#{{ option.value }}</span>
            </template>
            <template #footer="{ close }">
              <div class="flex shrink-0 items-center justify-between gap-3 border-t border-gray-200 p-3 dark:border-dark-700">
                <div class="flex flex-wrap gap-3">
                  <button type="button" class="text-xs text-primary-600" :disabled="accountSearchLoading[provider.value] || !availableAccountResults(provider.value).length" @click="selectCurrentResults(provider.value)">{{ t('admin.groups.shards.selectLoadedAccounts') }}</button>
                  <button type="button" class="text-xs text-gray-500 disabled:opacity-40" :disabled="!pendingAccounts(provider.value).length" @click="pendingSelection[provider.value] = []">{{ t('admin.groups.shards.clearSelectedAccounts') }}</button>
                </div>
                <button type="button" class="btn btn-primary btn-sm" :disabled="!pendingAccounts(provider.value).length || saving || disabled" @click="addPendingAccounts(provider.value); close()">{{ t('admin.groups.shards.addSelectedAccounts', { count: pendingAccounts(provider.value).length }) }}</button>
              </div>
            </template>
          </Select>
        </div>

                      <div
                        data-testid="account-pool-scroll"
                        class="mt-4 max-h-[min(24rem,50vh)] overflow-auto rounded-lg border border-gray-200 dark:border-dark-700 [scrollbar-gutter:stable]"
                      >
                        <div data-testid="account-pool-header" class="sticky top-0 z-10 grid grid-cols-[minmax(0,1fr)_5rem_2.5rem] gap-3 bg-gray-50 px-3 py-2 text-xs font-medium uppercase tracking-wide text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                          <span>{{ t("admin.groups.shards.accountName") }}</span>
                          <span>{{ t("admin.groups.shards.priority") }}</span>
                          <span class="text-right">{{ t("common.actions") }}</span>
                        </div>
                        <div v-if="selectedAccountsByProvider(provider.value).length === 0" class="px-3 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                          {{ t("admin.groups.shards.noProviderAccounts") }}
                        </div>
                        <div
                          v-for="draft in selectedAccountsByProvider(provider.value)"
                          :key="draft.account_id"
                          class="grid grid-cols-[minmax(0,1fr)_5rem_2.5rem] items-center gap-3 border-t border-gray-100 px-3 py-2 dark:border-dark-700"
                        >
                          <div class="min-w-0">
                            <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
                              {{ draft.name }}
                            </div>
                            <div class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                              <span class="font-mono">#{{ draft.account_id }}</span>
                              <span>{{ accountTypeLabel(draft.type) }}</span>
                              <span>{{ accountStatusLabel(draft.status) }}</span>
                            </div>
                          </div>
                          <input
                            v-model.number="draft.priority"
                            type="number"
                            step="1"
                            class="input input-sm !h-8 text-center tabular-nums"
                          />
                          <button
                            type="button"
                            class="justify-self-end rounded-lg p-2 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                            :title="t('common.remove')"
                            @click="removeAccount(draft.account_id)"
                          >
                            <Icon name="trash" size="sm" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </fieldset>
                </section>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api/admin";
import type { Account, AccountPlatform, AccountType, GroupPlatform, GroupShardAccount } from "@/types";
import { CONCRETE_PLATFORM_OPTIONS } from "@/constants/platforms";
import PlatformIcon from "@/components/common/PlatformIcon.vue";
import Icon from "@/components/icons/Icon.vue";
import Select from "@/components/common/Select.vue";

type AccountDraft = Pick<GroupShardAccount, "account_id" | "name" | "platform" | "type" | "status" | "priority">;
const props = defineProps<{
  groupId: number;
  platform: GroupPlatform;
  excludedAccountIds: number[];
  saving: boolean;
  disabled?: boolean;
  titleKey: string;
  hintKey: string;
}>();
const accountDrafts = defineModel<AccountDraft[]>({ required: true });
defineEmits<{ save: [] }>();
const { t } = useI18n();
const providerSections = computed(() => props.platform === "composite"
  ? [...CONCRETE_PLATFORM_OPTIONS]
  : CONCRETE_PLATFORM_OPTIONS.filter((provider) => provider.value === props.platform));
const blockedAccountIDs = computed(() => new Set([...props.excludedAccountIds, ...accountDrafts.value.map((account) => account.account_id)]));
const accountSearchResults = ref<Record<AccountPlatform, Account[]>>({} as Record<AccountPlatform, Account[]>);
const accountSearchLoading = ref<Record<AccountPlatform, boolean>>({} as Record<AccountPlatform, boolean>);
const accountSearchSequences: Partial<Record<AccountPlatform, number>> = {};
const accountSearchAbort: Partial<Record<AccountPlatform, AbortController>> = {};
const pendingSelection = ref<Partial<Record<AccountPlatform, Account[]>>>({});
function accountStatusLabel(status: string): string {
  return t(`admin.accounts.status.${status}`, status);
}

function accountTypeLabel(type: AccountType | string): string {
  if (type === "oauth") return t("admin.accounts.oauthType");
  if (type === "setup-token") return t("admin.accounts.setupToken");
  if (type === "apikey") return t("admin.accounts.apiKey");
  return type;
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

function selectedAccountsByProvider(platform: AccountPlatform): AccountDraft[] {
  return accountDrafts.value.filter((account) => account.platform === platform);
}

function availableAccountResults(platform: AccountPlatform): Account[] {
  return (accountSearchResults.value[platform] || []).filter((account) => !blockedAccountIDs.value.has(account.id));
}

async function loadAccountCandidates(platform: AccountPlatform, query: string): Promise<void> {
  if (props.groupId <= 0) return;
  const nextSequence = (accountSearchSequences[platform] || 0) + 1;
  accountSearchSequences[platform] = nextSequence;
  accountSearchAbort[platform]?.abort();
  const controller = new AbortController();
  accountSearchAbort[platform] = controller;
  accountSearchLoading.value[platform] = true;
  try {
    const accounts: Account[] = [];
    let page = 1;
    let pages = 1;
    do {
      const result = await adminAPI.accounts.list(
      page,
      100,
      {
        platform,
        group: String(props.groupId),
        search: query.trim() || undefined,
        lite: "true",
        sort_by: "priority",
        sort_order: "asc",
      },
      { signal: controller.signal },
    );
      if (controller.signal.aborted || accountSearchSequences[platform] !== nextSequence) return;
      accounts.push(...result.items);
      pages = result.pages || 1;
      page++;
    } while (page <= pages);
    accountSearchResults.value[platform] = accounts;
  } catch (error: unknown) {
    if (!controller.signal.aborted && accountSearchSequences[platform] === nextSequence) accountSearchResults.value[platform] = [];
  } finally {
    if (accountSearchSequences[platform] === nextSequence) accountSearchLoading.value[platform] = false;
  }
}

function pendingAccounts(platform: AccountPlatform): Account[] {
  return (pendingSelection.value[platform] || []).filter(account => !blockedAccountIDs.value.has(account.id));
}

function toggleAccount(platform: AccountPlatform, accountID: number): void {
  const selected = pendingAccounts(platform);
  const account = availableAccountResults(platform).find(item => item.id === accountID);
  pendingSelection.value[platform] = selected.some(item => item.id === accountID)
    ? selected.filter(item => item.id !== accountID)
    : account ? [...selected, account] : selected;
}

function selectCurrentResults(platform: AccountPlatform): void {
  pendingSelection.value[platform] = [...new Map([...pendingAccounts(platform), ...availableAccountResults(platform)].map(account => [account.id, account])).values()];
}

function addPendingAccounts(platform: AccountPlatform): void {
  accountDrafts.value = sortAccountDrafts([...accountDrafts.value, ...pendingAccounts(platform).map(account => ({ account_id: account.id, name: account.name, platform: account.platform, type: account.type, status: account.status, priority: 100 }))]);
  pendingSelection.value[platform] = [];
}

function removeAccount(accountID: number): void {
  accountDrafts.value = accountDrafts.value.filter((account) => account.account_id !== accountID);
}

function clearSearchState(): void {
  pendingSelection.value = {};
  for (const provider of CONCRETE_PLATFORM_OPTIONS) {
    const platform = provider.value;
    accountSearchAbort[platform]?.abort();
    accountSearchSequences[platform] = (accountSearchSequences[platform] || 0) + 1;
    accountSearchResults.value[platform] = [];
    accountSearchLoading.value[platform] = false;
  }
}
watch(() => [props.groupId, props.platform], clearSearchState);
onUnmounted(clearSearchState);
</script>
