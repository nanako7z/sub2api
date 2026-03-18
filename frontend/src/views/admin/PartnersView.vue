<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <!-- Left: Search + Filters -->
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.partners.search')"
              class="input"
              @input="handleSearch"
            />
          </div>
          <Select
            v-model="filters.status"
            :options="filterStatusOptions"
            class="w-36"
            @change="loadPartners"
          />

          <!-- Right: Action buttons -->
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              @click="loadPartners"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="showCreateDialog = true" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.partners.create') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="partners" :loading="loading">
          <template #cell-partner_name="{ value }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">{{ value }}</span>
          </template>

          <template #cell-referral_code="{ value }">
            <div class="flex items-center space-x-2">
              <code class="font-mono text-sm text-gray-900 dark:text-gray-100">{{ value }}</code>
              <button
                @click="copyCode(value)"
                :class="[
                  'flex items-center transition-colors',
                  copiedText === value
                    ? 'text-green-500'
                    : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'
                ]"
                :title="copiedText === value ? t('admin.partners.copied') : t('keys.copyToClipboard')"
              >
                <Icon v-if="copiedText !== value" name="copy" size="sm" :stroke-width="2" />
                <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
              </button>
            </div>
          </template>

          <template #cell-referred_users_count="{ value }">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ value }}</span>
          </template>

          <template #cell-pending_points="{ value }">
            <span class="text-sm font-medium text-amber-600 dark:text-amber-400">
              {{ value.toFixed(2) }}
            </span>
          </template>

          <template #cell-withdrawn_points="{ value }">
            <span class="text-sm font-medium text-green-600 dark:text-green-400">
              {{ value.toFixed(2) }}
            </span>
          </template>

          <template #cell-status="{ value }">
            <span :class="['badge', value === 'active' ? 'badge-success' : 'badge-gray']">
              {{ value === 'active' ? t('admin.partners.statusActive') : t('admin.partners.statusDisabled') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center space-x-1">
              <button
                @click="copyRegisterLink(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400"
                :title="t('admin.partners.copyRegisterLink')"
              >
                <Icon name="link" size="sm" />
              </button>
              <button
                @click="handleViewCommissions(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
                :title="t('admin.partners.viewCommissions')"
              >
                <Icon name="eye" size="sm" />
              </button>
              <button
                @click="handleWithdraw(row)"
                :class="[
                  'flex flex-col items-center gap-0.5 rounded-lg p-1.5 transition-colors',
                  row.pending_points > 0
                    ? 'text-gray-500 hover:bg-amber-50 hover:text-amber-600 dark:hover:bg-amber-900/20 dark:hover:text-amber-400'
                    : 'cursor-not-allowed text-gray-300 dark:text-dark-600'
                ]"
                :title="row.pending_points > 0 ? t('admin.partners.withdraw') : t('admin.partners.noPendingPoints')"
                :disabled="row.pending_points <= 0"
              >
                <Icon name="dollar" size="sm" />
              </button>
              <button
                @click="handleEdit(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-600 dark:hover:text-gray-300"
                :title="t('common.edit')"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                @click="handleDelete(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
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

    <!-- Create Dialog -->
    <BaseDialog
      :show="showCreateDialog"
      :title="t('admin.partners.create')"
      width="normal"
      @close="showCreateDialog = false"
    >
      <form id="create-partner-form" @submit.prevent="handleCreate" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.partners.partnerName') }} *</label>
          <input
            v-model="createForm.partner_name"
            type="text"
            required
            class="input"
            :placeholder="t('admin.partners.partnerNamePlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.partners.email') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('admin.partners.atLeastOneContact') }})</span>
          </label>
          <input
            v-model="createForm.email"
            type="email"
            class="input"
            :placeholder="t('admin.partners.emailPlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.partners.phone') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('admin.partners.atLeastOneContact') }})</span>
          </label>
          <input
            v-model="createForm.phone"
            type="text"
            class="input"
            :placeholder="t('admin.partners.phonePlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.partners.referralCode') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('admin.partners.autoGenerate') }})</span>
          </label>
          <input
            v-model="createForm.referral_code"
            type="text"
            class="input font-mono uppercase"
            :placeholder="t('admin.partners.referralCodePlaceholder')"
          />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">
              {{ t('admin.partners.signupBonus') }}
              <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
            </label>
            <input
              v-model.number="createForm.signup_bonus"
              type="number"
              min="0"
              step="0.01"
              class="input"
              :placeholder="t('admin.partners.signupBonusPlaceholder')"
            />
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.partners.signupBonusHint') }}</p>
          </div>
          <div>
            <label class="input-label">
              {{ t('admin.partners.maxPointsPerUser') }}
              <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
            </label>
            <input
              v-model.number="createForm.max_points_per_user"
              type="number"
              min="0"
              step="0.01"
              class="input"
              :placeholder="t('admin.partners.maxPointsPerUserPlaceholder')"
            />
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.partners.maxPointsPerUserHint') }}</p>
          </div>
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.partners.notes') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea
            v-model="createForm.notes"
            rows="2"
            class="input"
            :placeholder="t('admin.partners.notesPlaceholder')"
          ></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" @click="showCreateDialog = false" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="create-partner-form" :disabled="creating" class="btn btn-primary">
            {{ creating ? t('common.creating') : t('common.create') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Edit Dialog -->
    <BaseDialog
      :show="showEditDialog"
      :title="t('admin.partners.edit')"
      width="normal"
      @close="closeEditDialog"
    >
      <form id="edit-partner-form" @submit.prevent="handleUpdate" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.partners.partnerName') }} *</label>
          <input
            v-model="editForm.partner_name"
            type="text"
            required
            class="input"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.partners.email') }}</label>
          <input
            v-model="editForm.email"
            type="email"
            class="input"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.partners.phone') }}</label>
          <input
            v-model="editForm.phone"
            type="text"
            class="input"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.partners.status') }}</label>
          <Select v-model="editForm.status" :options="statusOptions" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">{{ t('admin.partners.signupBonus') }}</label>
            <input
              v-model.number="editForm.signup_bonus"
              type="number"
              min="0"
              step="0.01"
              class="input"
            />
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.partners.signupBonusHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.partners.maxPointsPerUser') }}</label>
            <input
              v-model.number="editForm.max_points_per_user"
              type="number"
              min="0"
              step="0.01"
              class="input"
            />
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.partners.maxPointsPerUserHint') }}</p>
          </div>
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.partners.notes') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea
            v-model="editForm.notes"
            rows="2"
            class="input"
          ></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" @click="closeEditDialog" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="edit-partner-form" :disabled="updating" class="btn btn-primary">
            {{ updating ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Withdraw Dialog -->
    <BaseDialog
      :show="showWithdrawDialog"
      :title="t('admin.partners.withdrawTitle')"
      width="normal"
      @close="showWithdrawDialog = false"
    >
      <div v-if="withdrawingPartner" class="space-y-4">
        <div class="rounded-lg bg-amber-50 p-4 dark:bg-amber-900/20">
          <p class="text-sm text-amber-800 dark:text-amber-200">
            {{ t('admin.partners.currentPending') }}:
            <span class="font-semibold">{{ withdrawingPartner.pending_points.toFixed(2) }}</span>
          </p>
        </div>
        <form id="withdraw-form" @submit.prevent="confirmWithdraw">
          <label class="input-label">{{ t('admin.partners.withdrawAmount') }}</label>
          <input
            v-model.number="withdrawAmount"
            type="number"
            min="0.01"
            :max="withdrawingPartner.pending_points"
            step="0.01"
            required
            class="input"
          />
        </form>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" @click="showWithdrawDialog = false" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="withdraw-form" :disabled="withdrawing" class="btn btn-primary">
            {{ withdrawing ? t('admin.partners.withdrawing') : t('admin.partners.confirmWithdraw') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Commissions Dialog -->
    <BaseDialog
      :show="showCommissionsDialog"
      :title="t('admin.partners.commissionRecords')"
      width="wide"
      @close="showCommissionsDialog = false"
    >
      <div v-if="commissionsLoading" class="flex items-center justify-center py-8">
        <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
      </div>
      <div v-else-if="commissions.length === 0" class="py-8 text-center text-gray-500 dark:text-gray-400">
        {{ t('admin.partners.noCommissions') }}
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="commission in commissions"
          :key="commission.id"
          class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-600"
        >
          <div class="flex items-center gap-3">
            <div class="flex h-8 w-8 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
              <Icon name="user" size="sm" class="text-green-600 dark:text-green-400" />
            </div>
            <div>
              <p class="text-sm font-medium text-gray-900 dark:text-white">
                {{ commission.referred_user_email || t('admin.partners.userPrefix', { id: commission.referred_user_id }) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ formatDateTime(commission.created_at) }}
              </p>
            </div>
          </div>
          <div class="text-right">
            <p class="text-sm font-medium text-green-600 dark:text-green-400">
              +{{ commission.points.toFixed(2) }} {{ t('admin.partners.points') }}
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.partners.sourceCost') }}: ${{ commission.source_cost.toFixed(2) }}
            </p>
          </div>
        </div>
        <!-- Commissions Pagination -->
        <div v-if="commissionsTotal > commissionsPageSize" class="mt-4">
          <Pagination
            :page="commissionsPage"
            :total="commissionsTotal"
            :page-size="commissionsPageSize"
            :page-size-options="[10, 20, 50]"
            @update:page="handleCommissionsPageChange"
            @update:page-size="(size: number) => { commissionsPageSize = size; commissionsPage = 1; loadCommissions() }"
          />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button type="button" @click="showCommissionsDialog = false" class="btn btn-secondary">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.partners.deletePartner')"
      :message="t('admin.partners.deleteConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { adminAPI } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import type { Partner, PartnerCommission } from '@/api/admin/partners'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

// State
const partners = ref<Partner[]>([])
const loading = ref(false)
const creating = ref(false)
const updating = ref(false)
const withdrawing = ref(false)
const searchQuery = ref('')
const copiedText = ref<string | null>(null)

const filters = reactive({
  status: ''
})

const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0
})

// Dialogs
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showDeleteDialog = ref(false)
const showWithdrawDialog = ref(false)
const showCommissionsDialog = ref(false)

const editingPartner = ref<Partner | null>(null)
const deletingPartner = ref<Partner | null>(null)
const withdrawingPartner = ref<Partner | null>(null)
const currentViewingPartner = ref<Partner | null>(null)

// Commissions
const commissions = ref<PartnerCommission[]>([])
const commissionsLoading = ref(false)
const commissionsPage = ref(1)
const commissionsPageSize = ref(20)
const commissionsTotal = ref(0)

// Withdraw
const withdrawAmount = ref(0)

// Forms
const createForm = reactive({
  partner_name: '',
  email: '',
  phone: '',
  referral_code: '',
  notes: '',
  signup_bonus: 0,
  max_points_per_user: 0
})

const editForm = reactive({
  partner_name: '',
  email: '',
  phone: '',
  status: 'active' as string,
  notes: '',
  signup_bonus: 0,
  max_points_per_user: 0
})

// Options
const filterStatusOptions = computed(() => [
  { value: '', label: t('admin.partners.allStatus') },
  { value: 'active', label: t('admin.partners.statusActive') },
  { value: 'disabled', label: t('admin.partners.statusDisabled') }
])

const statusOptions = computed(() => [
  { value: 'active', label: t('admin.partners.statusActive') },
  { value: 'disabled', label: t('admin.partners.statusDisabled') }
])

const columns = computed<Column[]>(() => [
  { key: 'partner_name', label: t('admin.partners.columns.partnerName') },
  { key: 'referral_code', label: t('admin.partners.columns.referralCode') },
  { key: 'referred_users_count', label: t('admin.partners.columns.referredUsersCount') },
  { key: 'pending_points', label: t('admin.partners.columns.pendingPoints'), sortable: true },
  { key: 'withdrawn_points', label: t('admin.partners.columns.withdrawnPoints'), sortable: true },
  { key: 'status', label: t('admin.partners.columns.status'), sortable: true },
  { key: 'created_at', label: t('admin.partners.columns.createdAt'), sortable: true },
  { key: 'actions', label: t('admin.partners.columns.actions') }
])

// API calls
let abortController: AbortController | null = null

const loadPartners = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentController = new AbortController()
  abortController = currentController
  loading.value = true

  try {
    const response = await adminAPI.partners.list(
      pagination.page,
      pagination.page_size,
      {
        status: filters.status || undefined,
        search: searchQuery.value || undefined
      }
    )
    if (currentController.signal.aborted) return

    partners.value = response.items
    pagination.total = response.total
  } catch (error: any) {
    if (currentController.signal.aborted || error?.name === 'AbortError') return
    appStore.showError(t('admin.partners.failedToLoad'))
    console.error('Error loading partners:', error)
  } finally {
    if (abortController === currentController && !currentController.signal.aborted) {
      loading.value = false
      abortController = null
    }
  }
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadPartners()
  }, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadPartners()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadPartners()
}

const copyCode = async (text: string) => {
  const success = await clipboardCopy(text, t('admin.partners.copied'))
  if (success) {
    copiedText.value = text
    setTimeout(() => {
      copiedText.value = null
    }, 2000)
  }
}

const copyRegisterLink = async (partner: Partner) => {
  const baseUrl = window.location.origin
  const registerLink = `${baseUrl}/register?promo=${encodeURIComponent(partner.referral_code)}`

  try {
    await navigator.clipboard.writeText(registerLink)
    appStore.showSuccess(t('admin.partners.registerLinkCopied'))
  } catch {
    const textArea = document.createElement('textarea')
    textArea.value = registerLink
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
    appStore.showSuccess(t('admin.partners.registerLinkCopied'))
  }
}

// Create
const handleCreate = async () => {
  if (!createForm.email && !createForm.phone) {
    appStore.showError(t('admin.partners.atLeastOneContactRequired'))
    return
  }
  creating.value = true
  try {
    await adminAPI.partners.create({
      partner_name: createForm.partner_name,
      email: createForm.email || undefined,
      phone: createForm.phone || undefined,
      referral_code: createForm.referral_code || undefined,
      notes: createForm.notes || undefined,
      signup_bonus: createForm.signup_bonus || undefined,
      max_points_per_user: createForm.max_points_per_user || undefined
    })
    appStore.showSuccess(t('admin.partners.created'))
    showCreateDialog.value = false
    resetCreateForm()
    loadPartners()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.partners.failedToCreate'))
  } finally {
    creating.value = false
  }
}

const resetCreateForm = () => {
  createForm.partner_name = ''
  createForm.email = ''
  createForm.phone = ''
  createForm.referral_code = ''
  createForm.notes = ''
  createForm.signup_bonus = 0
  createForm.max_points_per_user = 0
}

// Edit
const handleEdit = (partner: Partner) => {
  editingPartner.value = partner
  editForm.partner_name = partner.partner_name
  editForm.email = partner.email || ''
  editForm.phone = partner.phone || ''
  editForm.status = partner.status
  editForm.notes = partner.notes || ''
  editForm.signup_bonus = partner.signup_bonus || 0
  editForm.max_points_per_user = partner.max_points_per_user || 0
  showEditDialog.value = true
}

const closeEditDialog = () => {
  showEditDialog.value = false
  editingPartner.value = null
}

const handleUpdate = async () => {
  if (!editingPartner.value) return

  updating.value = true
  try {
    await adminAPI.partners.update(editingPartner.value.id, {
      partner_name: editForm.partner_name,
      email: editForm.email || undefined,
      phone: editForm.phone || undefined,
      status: editForm.status,
      notes: editForm.notes,
      signup_bonus: editForm.signup_bonus,
      max_points_per_user: editForm.max_points_per_user
    })
    appStore.showSuccess(t('admin.partners.updated'))
    closeEditDialog()
    loadPartners()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.partners.failedToUpdate'))
  } finally {
    updating.value = false
  }
}

// Withdraw
const handleWithdraw = (partner: Partner) => {
  withdrawingPartner.value = partner
  withdrawAmount.value = partner.pending_points
  showWithdrawDialog.value = true
}

const confirmWithdraw = async () => {
  if (!withdrawingPartner.value) return

  withdrawing.value = true
  try {
    await adminAPI.partners.withdrawPoints(withdrawingPartner.value.id, withdrawAmount.value)
    appStore.showSuccess(t('admin.partners.withdrawSuccess'))
    showWithdrawDialog.value = false
    withdrawingPartner.value = null
    loadPartners()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.partners.failedToWithdraw'))
  } finally {
    withdrawing.value = false
  }
}

// Delete
const handleDelete = (partner: Partner) => {
  deletingPartner.value = partner
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingPartner.value) return

  try {
    await adminAPI.partners.delete(deletingPartner.value.id)
    appStore.showSuccess(t('admin.partners.deleted'))
    showDeleteDialog.value = false
    deletingPartner.value = null
    loadPartners()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.partners.failedToDelete'))
  }
}

// View Commissions
const handleViewCommissions = async (partner: Partner) => {
  currentViewingPartner.value = partner
  showCommissionsDialog.value = true
  commissionsPage.value = 1
  await loadCommissions()
}

const loadCommissions = async () => {
  if (!currentViewingPartner.value) return
  commissionsLoading.value = true
  commissions.value = []

  try {
    const response = await adminAPI.partners.listCommissions(
      currentViewingPartner.value.id,
      commissionsPage.value,
      commissionsPageSize.value
    )
    commissions.value = response.items
    commissionsTotal.value = response.total
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.partners.failedToLoadCommissions'))
  } finally {
    commissionsLoading.value = false
  }
}

const handleCommissionsPageChange = (page: number) => {
  commissionsPage.value = page
  loadCommissions()
}

onMounted(() => {
  loadPartners()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
