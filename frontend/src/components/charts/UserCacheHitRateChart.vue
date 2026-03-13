<template>
  <div class="space-y-6">
    <!-- TOP users by token usage - cache hit rate -->
    <div class="card p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.topUsersByUsage') }}
      </h3>
      <div v-if="loading" class="flex h-64 items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <div v-else-if="topChartData" class="h-64">
        <Bar :data="topChartData" :options="barOptions" />
      </div>
      <div
        v-else
        class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>

    <!-- Lowest users by cache hit rate -->
    <div class="card p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.lowestCacheHitRate') }}
      </h3>
      <div v-if="loading" class="flex h-64 items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <div v-else-if="lowestChartData" class="h-64">
        <Bar :data="lowestChartData" :options="barOptions" />
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
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend
} from 'chart.js'
import { Bar } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { UserCacheHitRateTrendPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

interface Props {
  topUsersTrend: UserCacheHitRateTrendPoint[]
  lowestUsersTrend: UserCacheHitRateTrendPoint[]
  loading: boolean
}

const props = defineProps<Props>()
const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const colors = computed(() => ({
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  text: isDarkMode.value ? '#e5e7eb' : '#374151'
}))

// 12 distinct colors for up to 12 users
const userColors = [
  '#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#06b6d4',
  '#ec4899', '#84cc16', '#f97316', '#6366f1', '#14b8a6', '#e11d48'
]

const getDisplayName = (email: string, userId: number): string => {
  if (email && email.includes('@')) {
    return email.split('@')[0]
  }
  return `User#${userId}`
}

const buildChartData = (trendPoints: UserCacheHitRateTrendPoint[]) => {
  if (!trendPoints?.length) return null

  // Aggregate across all dates per user
  const userMap = new Map<
    number,
    {
      email: string
      inputTokens: number
      cacheCreationTokens: number
      cacheReadTokens: number
      totalRequests: number
      cacheHitRequests: number
    }
  >()

  for (const p of trendPoints) {
    const existing = userMap.get(p.user_id)
    if (existing) {
      existing.inputTokens += p.input_tokens
      existing.cacheCreationTokens += p.cache_creation_tokens
      existing.cacheReadTokens += p.cache_read_tokens
      existing.totalRequests += p.total_requests
      existing.cacheHitRequests += p.cache_hit_requests
    } else {
      userMap.set(p.user_id, {
        email: p.email,
        inputTokens: p.input_tokens,
        cacheCreationTokens: p.cache_creation_tokens,
        cacheReadTokens: p.cache_read_tokens,
        totalRequests: p.total_requests,
        cacheHitRequests: p.cache_hit_requests
      })
    }
  }

  const users = Array.from(userMap.entries())
  const labels = users.map(([userId, info]) => getDisplayName(info.email, userId))

  const tokenRates = users.map(([, info]) => {
    const denom = info.inputTokens + info.cacheCreationTokens + info.cacheReadTokens
    return denom > 0 ? +((info.cacheReadTokens / denom) * 100).toFixed(1) : 0
  })

  const requestRates = users.map(([, info]) => {
    return info.totalRequests > 0
      ? +((info.cacheHitRequests / info.totalRequests) * 100).toFixed(1)
      : 0
  })

  return {
    labels,
    datasets: [
      {
        label: t('admin.dashboard.tokenHitRate'),
        data: tokenRates,
        backgroundColor: userColors.slice(0, users.length).map((c) => `${c}cc`),
        borderColor: userColors.slice(0, users.length),
        borderWidth: 1
      },
      {
        label: t('admin.dashboard.requestHitRate'),
        data: requestRates,
        backgroundColor: userColors.slice(0, users.length).map((c) => `${c}55`),
        borderColor: userColors.slice(0, users.length).map((c) => `${c}99`),
        borderWidth: 1
      }
    ]
  }
}

const topChartData = computed(() => buildChartData(props.topUsersTrend))
const lowestChartData = computed(() => buildChartData(props.lowestUsersTrend))

const barOptions = computed(() => {
  const c = colors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    indexAxis: 'y' as const,
    plugins: {
      legend: {
        position: 'top' as const,
        labels: {
          color: c.text,
          usePointStyle: true,
          pointStyle: 'rect',
          padding: 12,
          font: { size: 11 }
        }
      },
      tooltip: {
        callbacks: {
          label: (ctx: any) => {
            const val = ctx.parsed.x
            if (val === null) return ''
            return `${ctx.dataset.label}: ${val.toFixed(1)}%`
          }
        }
      }
    },
    scales: {
      x: {
        beginAtZero: true,
        max: 100,
        grid: { color: c.grid },
        ticks: {
          color: c.text,
          font: { size: 10 },
          callback: (value: string | number) => `${value}%`
        }
      },
      y: {
        grid: { color: c.grid },
        ticks: { color: c.text, font: { size: 10 } }
      }
    }
  }
})
</script>
