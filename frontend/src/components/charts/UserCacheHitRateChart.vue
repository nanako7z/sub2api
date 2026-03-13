<template>
  <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
    <!-- TOP users by token usage - cache hit rate trend -->
    <div class="card p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.topUsersByUsage') }}
      </h3>
      <div v-if="loading" class="flex h-64 items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <div v-else-if="topChartData" class="h-64">
        <Line :data="topChartData" :options="lineOptions" />
      </div>
      <div
        v-else
        class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>

    <!-- Lowest users by cache hit rate - trend -->
    <div class="card p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.lowestCacheHitRate') }}
      </h3>
      <div v-if="loading" class="flex h-64 items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <div v-else-if="lowestChartData" class="h-64">
        <Line :data="lowestChartData" :options="lineOptions" />
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
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { UserCacheHitRateTrendPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

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

  // Extract unique dates (sorted) and unique users
  const dates = [...new Set(trendPoints.map((p) => p.date))].sort()
  const userMap = new Map<number, { email: string; data: Map<string, UserCacheHitRateTrendPoint> }>()

  for (const p of trendPoints) {
    if (!userMap.has(p.user_id)) {
      userMap.set(p.user_id, { email: p.email, data: new Map() })
    }
    userMap.get(p.user_id)!.data.set(p.date, p)
  }

  const datasets = Array.from(userMap.entries()).map(([userId, info], idx) => {
    const color = userColors[idx % userColors.length]
    return {
      label: getDisplayName(info.email, userId),
      data: dates.map((d) => {
        const point = info.data.get(d)
        return point ? +(point.token_cache_hit_rate * 100).toFixed(1) : null
      }),
      borderColor: color,
      backgroundColor: `${color}20`,
      fill: false,
      tension: 0.3,
      pointRadius: 2,
      borderWidth: 2
    }
  })

  return { labels: dates, datasets }
}

const topChartData = computed(() => buildChartData(props.topUsersTrend))
const lowestChartData = computed(() => buildChartData(props.lowestUsersTrend))

const lineOptions = computed(() => {
  const c = colors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: {
      intersect: false,
      mode: 'index' as const
    },
    plugins: {
      legend: {
        position: 'top' as const,
        labels: {
          color: c.text,
          usePointStyle: true,
          pointStyle: 'circle',
          padding: 10,
          font: { size: 10 },
          boxWidth: 8
        }
      },
      tooltip: {
        callbacks: {
          label: (ctx: any) => {
            const tokenRate = ctx.parsed.y
            if (tokenRate === null) return ''
            return `${ctx.dataset.label}: ${tokenRate.toFixed(1)}%`
          }
        }
      }
    },
    scales: {
      x: {
        grid: { color: c.grid },
        ticks: { color: c.text, font: { size: 10 } }
      },
      y: {
        beginAtZero: true,
        max: 100,
        grid: { color: c.grid },
        ticks: {
          color: c.text,
          font: { size: 10 },
          callback: (value: string | number) => `${value}%`
        }
      }
    }
  }
})
</script>
