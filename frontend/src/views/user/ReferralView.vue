<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <template v-else>
        <!-- Referral Code Card -->
        <div class="card overflow-hidden">
          <div class="bg-gradient-to-br from-primary-500 to-primary-600 px-6 py-8">
            <div class="text-center">
              <div
                class="mb-4 inline-flex h-16 w-16 items-center justify-center rounded-2xl bg-white/20 backdrop-blur-sm"
              >
                <svg class="h-8 w-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M7.217 10.907a2.25 2.25 0 100 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186l9.566-5.314m-9.566 7.5l9.566 5.314m0 0a2.25 2.25 0 103.935 2.186 2.25 2.25 0 00-3.935-2.186zm0-12.814a2.25 2.25 0 103.933-2.185 2.25 2.25 0 00-3.933 2.185z" />
                </svg>
              </div>
              <h2 class="text-2xl font-bold text-white">{{ t('referral.title') }}</h2>
              <p class="mt-2 text-sm text-primary-100">{{ t('referral.subtitle') }}</p>
            </div>

            <!-- Referral Code -->
            <div class="mt-6 space-y-3">
              <div v-if="stats.referral_code" class="rounded-xl bg-white/10 p-4 backdrop-blur-sm">
                <p class="mb-1 text-xs font-medium text-primary-200">{{ t('referral.yourCode') }}</p>
                <div class="flex items-center gap-2">
                  <span class="flex-1 font-mono text-lg font-bold text-white">{{ stats.referral_code }}</span>
                  <button
                    @click="copyToClipboard(stats.referral_code)"
                    class="rounded-lg bg-white/20 p-2 text-white transition-colors hover:bg-white/30"
                    :title="t('referral.copyCode')"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9.75a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
                    </svg>
                  </button>
                </div>
              </div>

              <div v-if="stats.referral_code" class="rounded-xl bg-white/10 p-4 backdrop-blur-sm">
                <p class="mb-1 text-xs font-medium text-primary-200">{{ t('referral.inviteLink') }}</p>
                <div class="flex items-center gap-2">
                  <span class="flex-1 truncate text-sm text-white/90">{{ inviteLink }}</span>
                  <button
                    @click="copyToClipboard(inviteLink)"
                    class="rounded-lg bg-white/20 p-2 text-white transition-colors hover:bg-white/30"
                    :title="t('referral.copyLink')"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m9.686-3.898a4.5 4.5 0 00-1.242-7.244l-4.5-4.5a4.5 4.5 0 00-6.364 6.364L4.343 8.81" />
                    </svg>
                  </button>
                </div>
              </div>

              <div v-else class="text-center">
                <button
                  @click="handleGenerateCode"
                  :disabled="generating"
                  class="rounded-xl bg-white/20 px-6 py-3 font-medium text-white transition-colors hover:bg-white/30 disabled:opacity-50"
                >
                  <svg v-if="generating" class="-ml-1 mr-2 inline h-4 w-4 animate-spin text-white" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  {{ t('referral.generateCode') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Stats Cards -->
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
                <svg class="h-5 w-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
                </svg>
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('referral.totalReferred') }}</p>
                <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ stats.total_referred }}</p>
              </div>
            </div>
          </div>

          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30">
                <svg class="h-5 w-5 text-emerald-600 dark:text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" />
                </svg>
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('referral.totalCommission') }}</p>
                <p class="text-2xl font-bold text-emerald-600 dark:text-emerald-400">${{ stats.total_commission.toFixed(2) }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- How It Works -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('referral.howItWorks') }}</h3>
          </div>
          <div class="p-6">
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div class="text-center">
                <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
                  <span class="text-lg font-bold">1</span>
                </div>
                <h4 class="font-medium text-gray-900 dark:text-white">{{ t('referral.step1Title') }}</h4>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('referral.step1Desc') }}</p>
              </div>
              <div class="text-center">
                <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
                  <span class="text-lg font-bold">2</span>
                </div>
                <h4 class="font-medium text-gray-900 dark:text-white">{{ t('referral.step2Title') }}</h4>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('referral.step2Desc') }}</p>
              </div>
              <div class="text-center">
                <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
                  <span class="text-lg font-bold">3</span>
                </div>
                <h4 class="font-medium text-gray-900 dark:text-white">{{ t('referral.step3Title') }}</h4>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('referral.step3Desc') }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Referred Users Table -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('referral.referredUsers') }}</h3>
          </div>
          <div class="overflow-x-auto">
            <table v-if="referredUsers.length > 0" class="w-full">
              <thead>
                <tr class="border-b border-gray-100 dark:border-dark-700">
                  <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('referral.userEmail') }}</th>
                  <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('referral.username') }}</th>
                  <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('referral.commissionEarned') }}</th>
                  <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('referral.joinedAt') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="user in referredUsers" :key="user.user_id" class="hover:bg-gray-50 dark:hover:bg-dark-800/50">
                  <td class="whitespace-nowrap px-6 py-4 text-sm text-gray-900 dark:text-gray-100">{{ maskEmail(user.email) }}</td>
                  <td class="whitespace-nowrap px-6 py-4 text-sm text-gray-600 dark:text-gray-400">{{ user.username || '-' }}</td>
                  <td class="whitespace-nowrap px-6 py-4 text-right text-sm font-medium text-emerald-600 dark:text-emerald-400">${{ user.total_commission.toFixed(2) }}</td>
                  <td class="whitespace-nowrap px-6 py-4 text-right text-sm text-gray-500 dark:text-gray-400">{{ formatDate(user.joined_at) }}</td>
                </tr>
              </tbody>
            </table>
            <div v-else class="px-6 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('referral.noReferredUsers') }}
            </div>
          </div>
          <!-- Pagination for referred users -->
          <div v-if="referredUsersTotal > usersPageSize" class="flex items-center justify-between border-t border-gray-100 px-6 py-3 dark:border-dark-700">
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('common.total') }}: {{ referredUsersTotal }}
            </p>
            <div class="flex gap-2">
              <button
                :disabled="usersPage <= 1"
                @click="usersPage--; loadReferredUsers()"
                class="btn btn-sm btn-ghost"
              >
                {{ t('common.back') }}
              </button>
              <button
                :disabled="usersPage * usersPageSize >= referredUsersTotal"
                @click="usersPage++; loadReferredUsers()"
                class="btn btn-sm btn-ghost"
              >
                {{ t('common.next') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Commission History Table -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('referral.commissionHistory') }}</h3>
          </div>
          <div class="overflow-x-auto">
            <table v-if="commissions.length > 0" class="w-full">
              <thead>
                <tr class="border-b border-gray-100 dark:border-dark-700">
                  <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('referral.commissionAmount') }}</th>
                  <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('referral.sourceCost') }}</th>
                  <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('referral.commissionRate') }}</th>
                  <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ t('referral.date') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="c in commissions" :key="c.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/50">
                  <td class="whitespace-nowrap px-6 py-4 text-sm font-medium text-emerald-600 dark:text-emerald-400">+${{ c.amount.toFixed(4) }}</td>
                  <td class="whitespace-nowrap px-6 py-4 text-right text-sm text-gray-600 dark:text-gray-400">${{ c.source_cost.toFixed(4) }}</td>
                  <td class="whitespace-nowrap px-6 py-4 text-right text-sm text-gray-600 dark:text-gray-400">{{ c.commission_rate.toFixed(1) }}%</td>
                  <td class="whitespace-nowrap px-6 py-4 text-right text-sm text-gray-500 dark:text-gray-400">{{ formatDate(c.created_at) }}</td>
                </tr>
              </tbody>
            </table>
            <div v-else class="px-6 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('referral.noCommissions') }}
            </div>
          </div>
          <!-- Pagination for commissions -->
          <div v-if="commissionsTotal > commissionsPageSize" class="flex items-center justify-between border-t border-gray-100 px-6 py-3 dark:border-dark-700">
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('common.total') }}: {{ commissionsTotal }}
            </p>
            <div class="flex gap-2">
              <button
                :disabled="commissionsPage <= 1"
                @click="commissionsPage--; loadCommissions()"
                class="btn btn-sm btn-ghost"
              >
                {{ t('common.back') }}
              </button>
              <button
                :disabled="commissionsPage * commissionsPageSize >= commissionsTotal"
                @click="commissionsPage++; loadCommissions()"
                class="btn btn-sm btn-ghost"
              >
                {{ t('common.next') }}
              </button>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import {
  getReferralStats,
  generateReferralCode,
  getReferredUsers,
  getReferralCommissions,
  type ReferralStats,
  type ReferredUser,
  type CommissionRecord
} from '@/api/referral'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const generating = ref(false)

const stats = ref<ReferralStats>({
  referral_code: '',
  total_referred: 0,
  total_commission: 0
})

const referredUsers = ref<ReferredUser[]>([])
const referredUsersTotal = ref(0)
const usersPage = ref(1)
const usersPageSize = 20

const commissions = ref<CommissionRecord[]>([])
const commissionsTotal = ref(0)
const commissionsPage = ref(1)
const commissionsPageSize = 20

const inviteLink = computed(() => {
  if (!stats.value.referral_code) return ''
  return `${window.location.origin}/register?promo=${stats.value.referral_code}`
})

async function loadStats() {
  try {
    const { data } = await getReferralStats()
    stats.value = data
  } catch (error) {
    console.error('Failed to load referral stats:', error)
  }
}

async function loadReferredUsers() {
  try {
    const { data } = await getReferredUsers({ page: usersPage.value, page_size: usersPageSize })
    referredUsers.value = data.items || []
    referredUsersTotal.value = data.total || 0
  } catch (error) {
    console.error('Failed to load referred users:', error)
  }
}

async function loadCommissions() {
  try {
    const { data } = await getReferralCommissions({ page: commissionsPage.value, page_size: commissionsPageSize })
    commissions.value = data.items || []
    commissionsTotal.value = data.total || 0
  } catch (error) {
    console.error('Failed to load commissions:', error)
  }
}

async function handleGenerateCode() {
  generating.value = true
  try {
    const { data } = await generateReferralCode()
    stats.value.referral_code = data.referral_code
    appStore.showSuccess(t('referral.codeGenerated'))
  } catch (error: any) {
    appStore.showError(error.message || t('referral.generateFailed'))
  } finally {
    generating.value = false
  }
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    appStore.showSuccess(t('common.copiedToClipboard'))
  }).catch(() => {
    appStore.showError(t('common.copyFailed'))
  })
}

function maskEmail(email: string): string {
  if (!email) return '-'
  const [local, domain] = email.split('@')
  if (!domain) return email
  if (local.length <= 2) return `${local[0]}***@${domain}`
  return `${local[0]}${local[1]}***@${domain}`
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  })
}

onMounted(async () => {
  loading.value = true
  await Promise.all([loadStats(), loadReferredUsers(), loadCommissions()])
  loading.value = false
})
</script>
