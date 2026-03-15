<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="relative flex min-h-screen flex-col overflow-hidden bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
  >
    <!-- Background Decorations -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute -right-40 -top-40 h-96 w-96 rounded-full bg-primary-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 -left-40 h-96 w-96 rounded-full bg-primary-500/15 blur-3xl"
      ></div>
      <div
        class="absolute left-1/3 top-1/4 h-72 w-72 rounded-full bg-primary-300/10 blur-3xl"
      ></div>
      <div
        class="absolute bottom-1/4 right-1/4 h-64 w-64 rounded-full bg-primary-400/10 blur-3xl"
      ></div>
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <!-- Logo -->
        <div class="flex items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-lg font-bold text-gray-900 dark:text-white">{{ siteName }}</span>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-3">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-full bg-gray-900 py-1 pl-1 pr-2.5 transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-gradient-to-br from-primary-400 to-primary-600 text-[10px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-medium text-white">{{ t('home.dashboard') }}</span>
            <svg
              class="h-3 w-3 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-gray-900 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6 py-16">
      <div class="mx-auto max-w-6xl">
        <!-- Hero Section - Left/Right Layout -->
        <div class="mb-16 flex flex-col items-center justify-between gap-12 lg:flex-row lg:gap-16">
          <!-- Left: Text Content -->
          <div class="flex-1 text-center lg:text-left">
            <h1
              class="mb-4 text-4xl font-extrabold tracking-tight text-gray-900 dark:text-white md:text-5xl lg:text-6xl"
            >
              订阅制价格<br>
              <span class="bg-gradient-to-r from-primary-500 to-primary-700 bg-clip-text text-transparent dark:from-primary-400 dark:to-primary-600">原生 API 体验</span>
            </h1>
            <p class="mb-6 max-w-lg text-lg leading-relaxed text-gray-600 dark:text-dark-300">
              多账号池化中转，将 Claude 订阅配额转化为稳定的 API 服务。<span class="font-semibold text-primary-600 dark:text-primary-400">完美支持 Claude Code</span>，智能故障转移让你无感切换。
            </p>

            <!-- Provider Badge -->
            <div class="mb-8">
              <span class="inline-flex items-center gap-2 rounded-full border border-orange-200 bg-orange-50 px-4 py-2 text-sm font-semibold text-orange-700 dark:border-orange-800/50 dark:bg-orange-900/20 dark:text-orange-400">
                <span class="flex h-5 w-5 items-center justify-center rounded-full bg-gradient-to-br from-orange-400 to-orange-500 text-[10px] font-bold text-white">C</span>
                Anthropic Claude API
              </span>
            </div>

            <!-- CTA Button -->
            <div class="flex flex-col items-center gap-3 sm:flex-row lg:justify-start">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="btn btn-primary px-8 py-3 text-base shadow-lg shadow-primary-500/30"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : '立即体验' }}
                <Icon name="arrowRight" size="md" class="ml-2" :stroke-width="2" />
              </router-link>
              <span class="text-sm text-gray-500 dark:text-dark-400">注册即用，按量付费</span>
            </div>
          </div>

          <!-- Right: Terminal Animation -->
          <div class="flex flex-1 justify-center lg:justify-end">
            <div class="terminal-container">
              <div class="terminal-window">
                <!-- Window header -->
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                  <span class="terminal-title">terminal</span>
                </div>
                <!-- Terminal content -->
                <div class="terminal-body">
                  <div class="code-line line-1">
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">curl</span>
                    <span class="code-flag">-X POST</span>
                    <span class="code-url">/v1/messages</span>
                  </div>
                  <div class="code-line line-2">
                    <span class="code-comment"># Routing to upstream...</span>
                  </div>
                  <div class="code-line line-3">
                    <span class="code-success">200 OK</span>
                    <span class="code-response">{ "content": "Hello!" }</span>
                  </div>
                  <div class="code-line line-4">
                    <span class="code-prompt">$</span>
                    <span class="cursor"></span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Stats Bar -->
        <div class="mb-16 grid grid-cols-2 gap-4 sm:grid-cols-4 sm:gap-6">
          <div class="rounded-2xl border border-gray-200/50 bg-white/60 p-5 text-center backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/60">
            <div class="text-2xl font-extrabold text-primary-600 dark:text-primary-400">Claude</div>
            <div class="mt-1 text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">专注平台</div>
          </div>
          <div class="rounded-2xl border border-gray-200/50 bg-white/60 p-5 text-center backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/60">
            <div class="text-2xl font-extrabold text-primary-600 dark:text-primary-400">N:1</div>
            <div class="mt-1 text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">多号池化</div>
          </div>
          <div class="rounded-2xl border border-gray-200/50 bg-white/60 p-5 text-center backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/60">
            <div class="text-2xl font-extrabold text-primary-600 dark:text-primary-400">&lt;500ms</div>
            <div class="mt-1 text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">故障切换</div>
          </div>
          <div class="rounded-2xl border border-gray-200/50 bg-white/60 p-5 text-center backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/60">
            <div class="text-2xl font-extrabold text-primary-600 dark:text-primary-400">99.9%</div>
            <div class="mt-1 text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">目标可用性</div>
          </div>
        </div>

        <!-- Features Grid (2x2) -->
        <div class="mb-16 grid gap-5 md:grid-cols-2">
          <!-- Feature 1 -->
          <div
            class="group rounded-2xl border border-gray-200/50 bg-white/60 p-7 backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-primary-500/10 dark:border-dark-700/50 dark:bg-dark-800/60"
          >
            <div class="mb-4 flex h-11 w-11 items-center justify-center rounded-xl bg-emerald-100 text-xl dark:bg-emerald-900/30">💰</div>
            <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">价格低至官方 API 几分之一</h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              基于订阅账号池化分发，按 token 精确计费。<strong class="font-semibold text-gray-700 dark:text-dark-300">Input / Output 分别计价</strong>，支持 Prompt Cache 差异费率，不多收一分钱。
            </p>
          </div>

          <!-- Feature 2 -->
          <div
            class="group rounded-2xl border border-gray-200/50 bg-white/60 p-7 backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-primary-500/10 dark:border-dark-700/50 dark:bg-dark-800/60"
          >
            <div class="mb-4 flex h-11 w-11 items-center justify-center rounded-xl bg-indigo-100 text-xl dark:bg-indigo-900/30">🛡️</div>
            <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">多层故障转移，可用性拉满</h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              同号重试 → 自动换号 → 智能限流解析，<strong class="font-semibold text-gray-700 dark:text-dark-300">多账号负载均衡</strong>。单点故障对用户完全透明，体感比单个官方 API 更稳定。
            </p>
          </div>

          <!-- Feature 3 -->
          <div
            class="group rounded-2xl border border-gray-200/50 bg-white/60 p-7 backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-primary-500/10 dark:border-dark-700/50 dark:bg-dark-800/60"
          >
            <div class="mb-4 flex h-11 w-11 items-center justify-center rounded-xl bg-amber-100 text-xl dark:bg-amber-900/30">🔌</div>
            <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">完全兼容 Anthropic 官方格式</h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              接口格式与官方 API 一致，<strong class="font-semibold text-gray-700 dark:text-dark-300">改个 Base URL 即可接入</strong>。Claude Code 专属优化快速路径，Streaming / SSE 全支持。
            </p>
          </div>

          <!-- Feature 4 -->
          <div
            class="group rounded-2xl border border-gray-200/50 bg-white/60 p-7 backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-primary-500/10 dark:border-dark-700/50 dark:bg-dark-800/60"
          >
            <div class="mb-4 flex h-11 w-11 items-center justify-center rounded-xl bg-pink-100 text-xl dark:bg-pink-900/30">⚡</div>
            <h3 class="mb-2 text-lg font-bold text-gray-900 dark:text-white">会话粘性，Cache 命中最大化</h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              基于亲和哈希的 Sticky Session，同一对话<strong class="font-semibold text-gray-700 dark:text-dark-300">始终路由同一账号</strong>。Prompt Cache 自动复用，响应更快、成本更低。
            </p>
          </div>
        </div>

        <!-- Value Propositions -->
        <div class="mb-16">
          <h2 class="mb-8 text-center text-2xl font-bold text-gray-900 dark:text-white">为什么选择 PureAPI</h2>
          <div class="grid gap-5 md:grid-cols-3">
            <div class="rounded-2xl border border-primary-200/50 bg-white/60 p-6 text-center backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 dark:border-primary-800/30 dark:bg-dark-800/60">
              <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 to-primary-600 shadow-lg shadow-primary-500/30">
                <Icon name="server" size="lg" class="text-white" />
              </div>
              <h4 class="mb-2 text-base font-bold text-gray-900 dark:text-white">专注 Anthropic 中转</h4>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">PureAPI 只做 Anthropic Claude，不分散精力，确保每个细节都做到极致。</p>
            </div>
            <div class="rounded-2xl border border-primary-200/50 bg-white/60 p-6 text-center backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 dark:border-primary-800/30 dark:bg-dark-800/60">
              <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 to-primary-600 shadow-lg shadow-primary-500/30">
                <Icon name="shield" size="lg" class="text-white" />
              </div>
              <h4 class="mb-2 text-base font-bold text-gray-900 dark:text-white">官方订阅上游</h4>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">坚持使用官方 Anthropic 订阅作为上游源，品质有保障，拒绝第三方转手。</p>
            </div>
            <div class="rounded-2xl border border-primary-200/50 bg-white/60 p-6 text-center backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 dark:border-primary-800/30 dark:bg-dark-800/60">
              <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 to-primary-600 shadow-lg shadow-primary-500/30">
                <Icon name="chart" size="lg" class="text-white" />
              </div>
              <h4 class="mb-2 text-base font-bold text-gray-900 dark:text-white">为重度用户而生</h4>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">为每天高频使用 AI 的开发者设计，提供稳定、极速、实惠的解决方案。</p>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/50 px-6 py-8 dark:border-dark-800/50">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }} — 更便宜、更稳定、更简单的 AI API
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'PureAPI')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
/* Terminal Container */
.terminal-container {
  position: relative;
  display: inline-block;
}

/* Terminal Window */
.terminal-window {
  width: 420px;
  background: linear-gradient(145deg, #1e293b 0%, #0f172a 100%);
  border-radius: 14px;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
  overflow: hidden;
  transform: perspective(1000px) rotateX(2deg) rotateY(-2deg);
  transition: transform 0.3s ease;
}

.terminal-window:hover {
  transform: perspective(1000px) rotateX(0deg) rotateY(0deg) translateY(-4px);
}

/* Terminal Header */
.terminal-header {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  background: rgba(30, 41, 59, 0.8);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.terminal-buttons {
  display: flex;
  gap: 8px;
}

.terminal-buttons span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.btn-close {
  background: #ef4444;
}
.btn-minimize {
  background: #eab308;
}
.btn-maximize {
  background: #22c55e;
}

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 12px;
  font-family: ui-monospace, monospace;
  color: #64748b;
  margin-right: 52px;
}

/* Terminal Body */
.terminal-body {
  padding: 20px 24px;
  font-family: ui-monospace, 'Fira Code', monospace;
  font-size: 14px;
  line-height: 2;
}

.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  opacity: 0;
  animation: line-appear 0.5s ease forwards;
}

.line-1 {
  animation-delay: 0.3s;
}
.line-2 {
  animation-delay: 1s;
}
.line-3 {
  animation-delay: 1.8s;
}
.line-4 {
  animation-delay: 2.5s;
}

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.code-prompt {
  color: #22c55e;
  font-weight: bold;
}
.code-cmd {
  color: #38bdf8;
}
.code-flag {
  color: #a78bfa;
}
.code-url {
  color: #14b8a6;
}
.code-comment {
  color: #64748b;
  font-style: italic;
}
.code-success {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.15);
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.code-response {
  color: #fbbf24;
}

/* Blinking Cursor */
.cursor {
  display: inline-block;
  width: 8px;
  height: 16px;
  background: #22c55e;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }
  51%,
  100% {
    opacity: 0;
  }
}

/* Dark mode adjustments */
:deep(.dark) .terminal-window {
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(20, 184, 166, 0.2),
    0 0 40px rgba(20, 184, 166, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

/* Responsive terminal */
@media (max-width: 768px) {
  .terminal-window {
    width: 100%;
    max-width: 420px;
  }
}
</style>
