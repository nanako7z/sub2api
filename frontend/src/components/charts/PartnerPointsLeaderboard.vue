<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.dashboard.partnerPointsLeaderboard') }}
    </h3>
    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="items.length > 0" class="max-h-48 overflow-y-auto">
      <table class="w-full text-xs">
        <thead>
          <tr class="text-gray-500 dark:text-gray-400">
            <th class="pb-2 text-left">#</th>
            <th class="pb-2 text-left">{{ t('admin.dashboard.partnerName') }}</th>
            <th class="pb-2 text-right">{{ t('admin.dashboard.partnerPoints') }}</th>
            <th class="pb-2 text-right">{{ t('admin.dashboard.commissionCount') }}</th>
            <th class="pb-2 text-right">{{ t('admin.dashboard.referredUsers') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(item, index) in items"
            :key="item.partner_id"
            class="border-t border-gray-100 dark:border-gray-700"
          >
            <td class="py-1.5 text-gray-600 dark:text-gray-400">{{ index + 1 }}</td>
            <td
              class="max-w-[120px] truncate py-1.5 font-medium text-gray-900 dark:text-white"
              :title="item.partner_name || item.email"
            >
              {{ item.partner_name || item.email || `Partner #${item.partner_id}` }}
            </td>
            <td class="py-1.5 text-right text-violet-600 dark:text-violet-400">
              {{ formatPoints(item.total_points) }}
            </td>
            <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
              {{ item.commission_count }}
            </td>
            <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
              {{ item.referred_users }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div
      v-else
      class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { PartnerPointsLeaderboardItem } from '@/types'

const { t } = useI18n()

defineProps<{
  items: PartnerPointsLeaderboardItem[]
  loading?: boolean
}>()

const formatPoints = (value: number): string => {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  else if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  else if (value >= 1) return value.toFixed(2)
  else if (value >= 0.01) return value.toFixed(3)
  return value.toFixed(4)
}
</script>
