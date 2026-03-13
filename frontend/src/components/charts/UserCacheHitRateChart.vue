<template>
  <div class="space-y-6">
    <!-- Overall cache hit rate bar chart -->
    <div class="card p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.overallCacheHitRate') }}
      </h3>
      <div v-if="loading" class="flex h-24 items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <div v-else-if="overallBarData" class="h-24">
        <Bar :data="overallBarData" :options="barOptions" />
      </div>
      <div
        v-else
        class="flex h-24 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>

    <!-- TOP users by token usage - cache miss rate trend -->
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

    <!-- Lowest users by cache hit rate - miss rate trend -->
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
  BarElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line, Bar } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint, UserCacheHitRateTrendPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

interface Props {
  trendData: TrendDataPoint[]
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

// Overall cache hit rate from trendData (token) and user cache stats (request)
const overallBarData = computed(() => {
  if (!props.trendData?.length) return null

  // Token hit rate from system-wide trendData
  let totalInput = 0
  let totalCreation = 0
  let totalRead = 0
  for (const d of props.trendData) {
    totalInput += d.input_tokens
    totalCreation += d.cache_creation_tokens
    totalRead += d.cache_read_tokens
  }
  const tokenDenom = totalInput + totalCreation + totalRead
  const tokenHitRate = tokenDenom > 0 ? +((totalRead / tokenDenom) * 100).toFixed(1) : 0

  // Request hit rate from user cache stats (deduplicated by user_id + date)
  const allPoints = [...(props.topUsersTrend || []), ...(props.lowestUsersTrend || [])]
  const seen = new Set<string>()
  let totalRequests = 0
  let cacheHitRequests = 0
  for (const p of allPoints) {
    const key = `${p.user_id}:${p.date}`
    if (seen.has(key)) continue
    seen.add(key)
    totalRequests += p.total_requests
    cacheHitRequests += p.cache_hit_requests
  }
  const requestHitRate =
    totalRequests > 0 ? +((cacheHitRequests / totalRequests) * 100).toFixed(1) : 0

  return {
    labels: [t('admin.dashboard.tokenHitRate'), t('admin.dashboard.requestHitRate')],
    datasets: [
      {
        data: [tokenHitRate, requestHitRate],
        backgroundColor: ['#3b82f6aa', '#10b981aa'],
        borderColor: ['#3b82f6', '#10b981'],
        borderWidth: 1
      }
    ]
  }
})

const barOptions = computed(() => {
  const c = colors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    indexAxis: 'y' as const,
    plugins: {
      legend: { display: false },
      tooltip: {
        callbacks: {
          label: (ctx: any) => `${ctx.parsed.x.toFixed(1)}%`
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
        grid: { display: false },
        ticks: { color: c.text, font: { size: 11 } }
      }
    }
  }
})

// Per-user miss rate line charts
const buildChartData = (trendPoints: UserCacheHitRateTrendPoint[]) => {
  if (!trendPoints?.length) return null

  const dates = [...new Set(trendPoints.map((p) => p.date))].sort()
  const userMap = new Map<
    number,
    { email: string; data: Map<string, UserCacheHitRateTrendPoint> }
  >()

  for (const p of trendPoints) {
    if (!userMap.has(p.user_id)) {
      userMap.set(p.user_id, { email: p.email, data: new Map() })
    }
    userMap.get(p.user_id)!.data.set(p.date, p)
  }

  const datasets: any[] = []
  Array.from(userMap.entries()).forEach(([userId, info], idx) => {
    const color = userColors[idx % userColors.length]
    const name = getDisplayName(info.email, userId)
    // Token miss rate - solid line
    datasets.push({
      label: `${name} (${t('admin.dashboard.tokenMissRate')})`,
      data: dates.map((d) => {
        const point = info.data.get(d)
        if (!point) return 0
        return +((1 - point.token_cache_hit_rate) * 100).toFixed(1)
      }),
      borderColor: color,
      backgroundColor: `${color}20`,
      fill: false,
      tension: 0.3,
      pointRadius: 2,
      borderWidth: 2,
      spanGaps: true
    })
    // Request miss rate - dashed line
    datasets.push({
      label: `${name} (${t('admin.dashboard.requestMissRate')})`,
      data: dates.map((d) => {
        const point = info.data.get(d)
        if (!point) return 0
        return +((1 - point.request_cache_hit_rate) * 100).toFixed(1)
      }),
      borderColor: color,
      backgroundColor: `${color}20`,
      fill: false,
      tension: 0.3,
      pointRadius: 1,
      borderWidth: 1.5,
      borderDash: [4, 3],
      spanGaps: true
    })
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
            const val = ctx.parsed.y
            if (val === null) return ''
            return `${ctx.dataset.label}: ${val.toFixed(1)}%`
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
