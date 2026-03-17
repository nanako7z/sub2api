<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.dashboard.spendingLeaderboard') }}
    </h3>
    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="items.length > 0" class="max-h-48 overflow-y-auto">
      <table class="w-full text-xs">
        <thead>
          <tr class="text-gray-500 dark:text-gray-400">
            <th class="pb-2 text-left">#</th>
            <th class="pb-2 text-left">{{ t('admin.dashboard.user') }}</th>
            <th class="pb-2 text-right">{{ t('admin.dashboard.actual') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(item, index) in items"
            :key="item.user_id"
            class="border-t border-gray-100 dark:border-gray-700"
          >
            <td class="py-1.5 text-gray-600 dark:text-gray-400">{{ index + 1 }}</td>
            <td
              class="max-w-[120px] truncate py-1.5 font-medium text-gray-900 dark:text-white"
              :title="item.email"
            >
              {{ item.email || `User #${item.user_id}` }}
            </td>
            <td class="py-1.5 text-right text-amber-600 dark:text-amber-400">
              ${{ formatCost(item.actual_cost) }}
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
import type { UserSpendingRankingItem } from '@/types'

const { t } = useI18n()

defineProps<{
  items: UserSpendingRankingItem[]
  loading?: boolean
}>()

const formatCost = (value: number): string => {
  if (value >= 1000) return (value / 1000).toFixed(2) + 'K'
  else if (value >= 1) return value.toFixed(2)
  else if (value >= 0.01) return value.toFixed(3)
  return value.toFixed(4)
}


</script>
