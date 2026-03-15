<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.dashboard.platformCacheHitRateTrend') }}
    </h3>
    <div v-if="loading" class="flex h-64 items-center justify-center">
      <LoadingSpinner size="md" />
    </div>
    <div v-else-if="chartData" class="h-64">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div
      v-else
      class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
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
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

interface Props {
  trendData: TrendDataPoint[]
  loading: boolean
}

const props = defineProps<Props>()
const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const colors = computed(() => ({
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  text: isDarkMode.value ? '#e5e7eb' : '#374151'
}))

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  const dates = props.trendData.map((d) => d.date)

  const tokenHitRates = props.trendData.map((d) => {
    const denom = d.input_tokens + d.cache_creation_tokens + d.cache_read_tokens
    return denom > 0 ? +((d.cache_read_tokens / denom) * 100).toFixed(1) : 0
  })

  const requestHitRates = props.trendData.map((d) => {
    return d.requests > 0
      ? +(((d.cache_hit_requests ?? 0) / d.requests) * 100).toFixed(1)
      : 0
  })

  return {
    labels: dates,
    datasets: [
      {
        label: t('admin.dashboard.tokenCacheHitRate'),
        data: tokenHitRates,
        borderColor: '#3b82f6',
        backgroundColor: 'rgba(59, 130, 246, 0.1)',
        fill: true,
        tension: 0.3,
        pointRadius: 3,
        pointHoverRadius: 5,
        borderWidth: 2
      },
      {
        label: t('admin.dashboard.requestCacheHitRate'),
        data: requestHitRates,
        borderColor: '#10b981',
        backgroundColor: 'rgba(16, 185, 129, 0.1)',
        fill: true,
        tension: 0.3,
        pointRadius: 3,
        pointHoverRadius: 5,
        borderWidth: 2,
        borderDash: [5, 3]
      }
    ]
  }
})

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
          padding: 15,
          font: { size: 11 }
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
