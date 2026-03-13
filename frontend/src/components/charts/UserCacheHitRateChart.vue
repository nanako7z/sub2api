<template>
  <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
    <!-- TOP 12 users by token usage -->
    <div class="card p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.topUsersByUsage') }}
      </h3>
      <div v-if="loading" class="flex h-64 items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <div v-else-if="topChartData" class="h-64">
        <Bar :data="topChartData" :options="horizontalBarOptions" />
      </div>
      <div
        v-else
        class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>

    <!-- Lowest 12 users by cache hit rate -->
    <div class="card p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.lowestCacheHitRate') }}
      </h3>
      <div v-if="loading" class="flex h-64 items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <div v-else-if="lowestChartData" class="h-64">
        <Bar :data="lowestChartData" :options="horizontalBarOptions" />
      </div>
      <div
        v-else
        class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  BarElement,
  CategoryScale,
  LinearScale,
  Title,
  Tooltip,
  Legend
} from 'chart.js'
import { Bar } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { UserCacheHitRateStat } from '@/types'

ChartJS.register(BarElement, CategoryScale, LinearScale, Title, Tooltip, Legend)

interface Props {
  topUsers: UserCacheHitRateStat[]
  lowestUsers: UserCacheHitRateStat[]
  loading: boolean
}

const props = defineProps<Props>()
const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const colors = computed(() => ({
  grid: isDarkMode.value ? '#374151' : '#f3f4f6',
  text: isDarkMode.value ? '#9ca3af' : '#6b7280'
}))

const getDisplayName = (email: string, userId: number): string => {
  if (email && email.includes('@')) {
    return email.split('@')[0]
  }
  return `User#${userId}`
}

const topChartData = computed(() => {
  if (!props.topUsers?.length) return null
  return {
    labels: props.topUsers.map((u) => getDisplayName(u.email, u.user_id)),
    datasets: [
      {
        label: t('admin.dashboard.tokenHitRate'),
        data: props.topUsers.map((u) => +(u.token_cache_hit_rate * 100).toFixed(1)),
        backgroundColor: '#3b82f6',
        borderRadius: 4,
        barPercentage: 0.5
      },
      {
        label: t('admin.dashboard.requestHitRate'),
        data: props.topUsers.map((u) => +(u.request_cache_hit_rate * 100).toFixed(1)),
        backgroundColor: '#10b981',
        borderRadius: 4,
        barPercentage: 0.5
      }
    ]
  }
})

const lowestChartData = computed(() => {
  if (!props.lowestUsers?.length) return null
  return {
    labels: props.lowestUsers.map((u) => getDisplayName(u.email, u.user_id)),
    datasets: [
      {
        label: t('admin.dashboard.tokenHitRate'),
        data: props.lowestUsers.map((u) => +(u.token_cache_hit_rate * 100).toFixed(1)),
        backgroundColor: '#ef4444',
        borderRadius: 4,
        barPercentage: 0.6
      }
    ]
  }
})

const horizontalBarOptions = computed(() => {
  const c = colors.value
  return {
    indexAxis: 'y' as const,
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: true,
        position: 'top' as const,
        labels: { color: c.text, font: { size: 11 }, boxWidth: 12 }
      },
      tooltip: {
        callbacks: {
          label: (ctx: { dataset: { label?: string }; parsed: { x: number | null } }) =>
            `${ctx.dataset.label}: ${(ctx.parsed.x ?? 0).toFixed(1)}%`
        }
      }
    },
    scales: {
      x: {
        beginAtZero: true,
        max: 100,
        grid: { color: c.grid, borderDash: [4, 4] },
        ticks: {
          color: c.text,
          font: { size: 10 },
          callback: (value: string | number) => `${value}%`
        }
      },
      y: {
        grid: { display: false },
        ticks: { color: c.text, font: { size: 10 } }
      }
    }
  }
})
</script>
