<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-5">
    <!-- ── Page Header ────────────────────────────────────────────────────────── -->
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <div class="flex items-center gap-2 text-[11px] uppercase tracking-[0.14em] text-muted font-mono">
          <span>Admin</span>
          <ChevronRightIcon class="w-3 h-3" />
          <span class="text-foreground/70">AI Settings</span>
        </div>
        <h1 class="mt-1 text-2xl font-bold tracking-tight">AI Settings</h1>
        <p class="mt-1 text-sm text-muted">Configure the AI provider used for artifact analysis and intelligence features.</p>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <button
          type="button"
          :disabled="testing || loading"
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium border border-border bg-surface text-foreground hover:bg-surface-hover transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          @click="testConnection"
        >
          <ArrowPathIcon class="w-4 h-4" :class="{ 'animate-spin': testing }" />
          Test Connection
        </button>
        <button
          type="button"
          :disabled="saving || loading"
          class="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-medium bg-primary text-primary-contrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
          @click="save"
        >
          <ArrowPathIcon v-if="saving" class="w-4 h-4 animate-spin" />
          <CheckIcon v-else class="w-4 h-4" />
          {{ saving ? 'Saving…' : 'Save Changes' }}
        </button>
      </div>
    </div>

    <!-- ── Loading state ─────────────────────────────────────────────────────── -->
    <div v-if="loading" class="bg-surface rounded-xl border border-border p-12 flex flex-col items-center gap-3 text-muted">
      <ArrowPathIcon class="w-6 h-6 animate-spin" />
      <span class="text-sm">Loading configuration…</span>
    </div>

    <!-- ── Form body ─────────────────────────────────────────────────────────── -->
    <template v-else>

      <!-- ── Enable Toggle Banner ─────────────────────────────────────────────── -->
      <div class="bg-surface rounded-xl border border-border p-5 flex items-start gap-4">
        <button
          type="button"
          role="switch"
          :aria-checked="form.enabled"
          class="relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors duration-200 mt-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
          :class="form.enabled ? 'bg-primary' : 'bg-border'"
          @click="form.enabled = !form.enabled"
        >
          <span
            class="inline-block h-5 w-5 rounded-full bg-white shadow-sm transition-transform duration-200 mt-0.5"
            :class="form.enabled ? 'translate-x-5' : 'translate-x-0.5'"
          />
        </button>
        <div class="min-w-0">
          <p class="text-sm font-semibold text-foreground leading-tight">Enable AI Analysis</p>
          <p class="text-[12px] text-muted mt-0.5 leading-relaxed">
            When enabled, OpenArtifact will automatically analyse uploaded artifacts using the configured AI provider
            to extract license information, security notes, and quarantine recommendations.
          </p>
        </div>
        <span
          class="shrink-0 inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-semibold"
          :class="form.enabled ? 'bg-success-soft text-success' : 'bg-surface-hover text-muted'"
        >
          <span class="w-1.5 h-1.5 rounded-full" :class="form.enabled ? 'bg-success' : 'bg-muted'" />
          {{ form.enabled ? 'Enabled' : 'Disabled' }}
        </span>
      </div>

      <!-- ── Form content (fades when disabled) ───────────────────────────────── -->
      <div class="space-y-4 transition-opacity duration-200" :class="{ 'opacity-40 pointer-events-none': !form.enabled }">

        <!-- ── Provider Picker ───────────────────────────────────────────────── -->
        <div class="bg-surface rounded-xl border border-border p-5 space-y-3">
          <div>
            <h3 class="text-sm font-semibold text-foreground">Provider</h3>
            <p class="text-[11px] text-muted mt-0.5">Select the AI backend that will power artifact analysis.</p>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="(meta, key) in PROVIDERS"
              :key="key"
              type="button"
              class="relative text-left p-3 rounded-lg border overflow-hidden transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
              :class="form.provider === key
                ? 'border-primary bg-primary-soft/40 ring-1 ring-primary/30'
                : 'border-border bg-surface hover:bg-surface-hover cursor-pointer'"
              @click="selectProvider(key)"
            >
              <!-- Accent bar -->
              <span
                class="absolute left-0 top-0 bottom-0 w-[3px] rounded-l-lg"
                :style="{ background: meta.color }"
              />
              <div class="pl-3 flex items-start gap-2.5">
                <div
                  class="w-7 h-7 rounded-md flex items-center justify-center shrink-0 mt-0.5"
                  :style="{ background: meta.color + '20' }"
                >
                  <svg
                    v-if="aiBrandIcon(String(key))"
                    class="w-4 h-4"
                    :style="{ color: meta.color }"
                    aria-hidden="true"
                    :viewBox="aiBrandIcon(String(key))!.viewBox"
                    fill="currentColor"
                  >
                    <path :d="aiBrandIcon(String(key))!.path" />
                  </svg>
                  <component v-else :is="meta.icon" class="w-4 h-4" :style="{ color: meta.color }" />
                </div>
                <div class="min-w-0">
                  <p class="text-[12px] font-semibold text-foreground leading-tight">{{ meta.label }}</p>
                  <p class="text-[11px] text-muted leading-snug mt-0.5">{{ meta.desc }}</p>
                </div>
              </div>
            </button>
          </div>
        </div>

        <!-- ── Configuration Fields ──────────────────────────────────────────── -->
        <div class="bg-surface rounded-xl border border-border p-5 space-y-4">
          <div>
            <h3 class="text-sm font-semibold text-foreground">Configuration</h3>
            <p class="text-[11px] text-muted mt-0.5">Connection details and model parameters.</p>
          </div>

          <div class="grid grid-cols-2 gap-x-4 gap-y-4">
            <!-- Base URL -->
            <div class="col-span-2 sm:col-span-1">
              <label class="block text-xs font-semibold text-foreground mb-1.5">
                Base URL <span class="text-danger ml-0.5">*</span>
              </label>
              <input
                v-model="form.base_url"
                type="text"
                placeholder="https://api.openai.com/v1"
                class="w-full px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
              />
              <p class="text-[11px] text-muted mt-1">OpenAI-compatible endpoint URL.</p>
            </div>

            <!-- API Key -->
            <div class="col-span-2 sm:col-span-1">
              <label class="block text-xs font-semibold text-foreground mb-1.5">API Key</label>
              <div class="relative">
                <input
                  v-model="form.api_key"
                  :type="apiKeyRevealed ? 'text' : 'password'"
                  placeholder="••••••••"
                  class="w-full px-3 py-2 pr-12 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                />
                <button
                  type="button"
                  class="absolute right-2.5 top-1/2 -translate-y-1/2 text-[11px] font-medium text-muted hover:text-foreground transition-colors"
                  @click="apiKeyRevealed = !apiKeyRevealed"
                >
                  {{ apiKeyRevealed ? 'hide' : 'show' }}
                </button>
              </div>
              <p class="text-[11px] text-muted mt-1">Leave unchanged (••••••••) to keep the existing key.</p>
            </div>

            <!-- Model -->
            <div class="col-span-2 sm:col-span-1">
              <label class="block text-xs font-semibold text-foreground mb-1.5">Model</label>
              <div class="flex gap-2">
                <div class="relative flex-1 min-w-0">
                  <input
                    v-model="form.model"
                    type="text"
                    list="model-suggestions"
                    :placeholder="PROVIDERS[form.provider]?.defaultModels?.[0] ?? 'gpt-4o'"
                    class="w-full px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                  />
                  <datalist id="model-suggestions">
                    <option v-for="m in modelSuggestions" :key="m" :value="m" />
                  </datalist>
                </div>
                <button
                  type="button"
                  :disabled="loadingModels"
                  class="shrink-0 inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium border border-border bg-surface text-foreground hover:bg-surface-hover transition-colors disabled:opacity-40 disabled:cursor-not-allowed whitespace-nowrap"
                  title="Fetch available models from the configured endpoint"
                  @click="loadModels"
                >
                  <ArrowPathIcon class="w-3.5 h-3.5" :class="{ 'animate-spin': loadingModels }" />
                  Load
                </button>
              </div>
              <p v-if="availableModels.length > 0" class="text-[11px] text-muted mt-1">
                {{ availableModels.length }} models available — type or select from suggestions.
              </p>
              <p v-else class="text-[11px] text-muted mt-1">Type a model name or click Load to fetch from the API.</p>
            </div>

            <!-- Max Tokens -->
            <div class="col-span-2 sm:col-span-1">
              <label class="block text-xs font-semibold text-foreground mb-1.5">Max Tokens</label>
              <input
                v-model.number="form.max_tokens"
                type="number"
                min="128"
                max="128000"
                step="128"
                class="w-full px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
              />
              <p class="text-[11px] text-muted mt-1">Maximum tokens in the model response.</p>
            </div>

            <!-- Timeout -->
            <div class="col-span-2 sm:col-span-1">
              <label class="block text-xs font-semibold text-foreground mb-1.5">Timeout (seconds)</label>
              <input
                v-model.number="form.timeout_secs"
                type="number"
                min="5"
                max="300"
                step="5"
                class="w-full px-3 py-2 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/50 focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
              />
              <p class="text-[11px] text-muted mt-1">Request timeout before the connection is aborted.</p>
            </div>
          </div>
        </div>

        <!-- ── System Prompt ───────────────────────────────────────────────────── -->
        <div class="bg-surface rounded-xl border border-border p-5 space-y-3">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h3 class="text-sm font-semibold text-foreground">System Prompt</h3>
              <p class="text-[11px] text-muted mt-0.5">
                Instructions sent to the model before the artifact analysis request. Leave empty to use the built-in default.
              </p>
            </div>
            <button
              v-if="form.system_prompt"
              type="button"
              class="shrink-0 text-[11px] font-medium text-muted hover:text-danger transition-colors"
              @click="form.system_prompt = ''"
            >
              Reset to default
            </button>
          </div>
          <textarea
            v-model="form.system_prompt"
            rows="6"
            placeholder="You are an expert in software artifact analysis. Analyse the provided artifact manifest and return a structured JSON response with: summary (string), tags (array), license_status (SAFE|REVIEW_REQUIRED|UNKNOWN), license_name (string), security_notes (string), quarantine_recommended (bool)."
            class="w-full px-3 py-2.5 rounded-lg border border-border bg-bg text-foreground text-sm font-mono placeholder:text-muted/40 focus:outline-none focus:ring-2 focus:ring-primary resize-none transition-colors leading-relaxed"
          />
        </div>

      </div><!-- end form content -->

      <!-- ── Test Result Panel ───────────────────────────────────────────────── -->
      <Transition
        enter-active-class="transition-all duration-300 ease-out"
        enter-from-class="opacity-0 -translate-y-2"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition-all duration-200 ease-in"
        leave-from-class="opacity-100 translate-y-0"
        leave-to-class="opacity-0 -translate-y-1"
      >
        <div
          v-if="testResult !== null"
          class="rounded-xl border p-4"
          :class="testResult.ok
            ? 'border-success/30 bg-success-soft'
            : 'border-danger/30 bg-danger-soft'"
        >
          <div class="flex items-start gap-3">
            <div
              class="w-8 h-8 rounded-lg flex items-center justify-center shrink-0 mt-0.5"
              :class="testResult.ok ? 'bg-success/20' : 'bg-danger/20'"
            >
              <CheckCircleIcon v-if="testResult.ok" class="w-4 h-4 text-success" />
              <ExclamationCircleIcon v-else class="w-4 h-4 text-danger" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="text-sm font-semibold" :class="testResult.ok ? 'text-success' : 'text-danger'">
                {{ testResult.ok ? 'Connection successful' : 'Connection failed' }}
              </p>
              <p v-if="testResult.error" class="text-[12px] mt-0.5 font-mono break-all" :class="testResult.ok ? 'text-success/80' : 'text-danger/80'">
                {{ testResult.error }}
              </p>
              <!-- Available models pills -->
              <div v-if="testResult.ok && testResult.models && testResult.models.length > 0" class="flex flex-wrap gap-1.5 mt-2">
                <span
                  v-for="m in testResult.models"
                  :key="m"
                  class="inline-flex items-center px-2 py-0.5 rounded-md bg-success/15 text-success text-[11px] font-mono font-medium border border-success/20 cursor-pointer hover:bg-success/25 transition-colors"
                  :title="'Click to use ' + m"
                  @click="form.model = m"
                >
                  {{ m }}
                </span>
              </div>
              <p v-if="testResult.ok && testResult.models && testResult.models.length > 0" class="text-[11px] text-success/70 mt-1.5">
                Click a model pill to select it.
              </p>
            </div>
            <button
              type="button"
              class="shrink-0 p-1.5 rounded-md text-muted hover:text-foreground hover:bg-surface-hover/60 transition-colors"
              @click="testResult = null"
            >
              <XMarkIcon class="w-4 h-4" />
            </button>
          </div>
        </div>
      </Transition>

    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import type { Component } from 'vue'
import {
  ChevronRightIcon,
  ArrowPathIcon,
  CheckIcon,
  CheckCircleIcon,
  ExclamationCircleIcon,
  XMarkIcon,
  SparklesIcon,
  CloudIcon,
  PuzzlePieceIcon,
} from '@heroicons/vue/24/outline'
import { useToastStore } from '../../stores/toast'
import { getAIConfig, saveAIConfig, testAIConfig } from '../../api/adminAi'
import type { AIConfig, TestResult } from '../../api/adminAi'
import { aiBrandIcon, type BrandIcon } from '../../lib/brandIcons'

// ─── Provider Registry ─────────────────────────────────────────────────────────

interface ProviderMeta {
  label: string
  desc: string
  color: string
  icon?: Component
  brand?: BrandIcon
  defaultUrl: string
  defaultModels: string[]
}

const PROVIDERS: Record<string, ProviderMeta> = {
  openai: {
    label: 'OpenAI',
    desc: 'GPT-4o, GPT-4o-mini, o1',
    color: '#10a37f',
    icon: SparklesIcon,
    defaultUrl: 'https://api.openai.com/v1',
    defaultModels: ['gpt-4o', 'gpt-4o-mini', 'o1-mini'],
  },
  azure: {
    label: 'Azure OpenAI',
    desc: 'Azure-hosted OpenAI models',
    color: '#0078d4',
    icon: CloudIcon,
    defaultUrl: 'https://{resource}.openai.azure.com/openai/deployments/{deployment}',
    defaultModels: ['gpt-4o', 'gpt-35-turbo'],
  },
  ollama: {
    label: 'Ollama',
    desc: 'Local models via Ollama',
    color: '#f97316',
    defaultUrl: 'http://localhost:11434/v1',
    defaultModels: ['llama3.2', 'mistral', 'codellama'],
  },
  custom: {
    label: 'Custom / OpenAI-compatible',
    desc: 'Any OpenAI-compatible endpoint',
    color: '#8b5cf6',
    icon: PuzzlePieceIcon,
    defaultUrl: '',
    defaultModels: [],
  },
}

// ─── State ─────────────────────────────────────────────────────────────────────

const toasts = useToastStore()

const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const loadingModels = ref(false)
const apiKeyRevealed = ref(false)
const availableModels = ref<string[]>([])
const testResult = ref<TestResult | null>(null)

const form = reactive<AIConfig>({
  enabled: false,
  provider: 'openai',
  base_url: '',
  api_key: '',
  model: '',
  max_tokens: 2048,
  timeout_secs: 30,
  system_prompt: '',
})

// ─── Computed ──────────────────────────────────────────────────────────────────

const modelSuggestions = computed<string[]>(() => {
  // Merge loaded models with provider defaults, deduplicated
  const defaults = PROVIDERS[form.provider]?.defaultModels ?? []
  const combined = [...new Set([...availableModels.value, ...defaults])]
  return combined
})

// ─── Actions ───────────────────────────────────────────────────────────────────

function selectProvider(key: string): void {
  const prev = form.provider
  form.provider = key
  availableModels.value = []
  // Auto-set base URL if it was the default for the previous provider or is empty
  const prevDefault = PROVIDERS[prev]?.defaultUrl ?? ''
  const newDefault = PROVIDERS[key]?.defaultUrl ?? ''
  if (!form.base_url || form.base_url === prevDefault) {
    form.base_url = newDefault
  }
}

async function loadModels(): Promise<void> {
  loadingModels.value = true
  try {
    const result = await testAIConfig({ ...form })
    if (result.ok && result.models && result.models.length > 0) {
      availableModels.value = result.models
      toasts.success(`Loaded ${result.models.length} models`)
    } else if (!result.ok) {
      toasts.error(result.error ?? 'Failed to load models')
    } else {
      toasts.info?.('No models returned by the endpoint')
    }
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to load models')
  } finally {
    loadingModels.value = false
  }
}

async function testConnection(): Promise<void> {
  testing.value = true
  testResult.value = null
  try {
    const result = await testAIConfig({ ...form })
    testResult.value = result
    if (result.ok && result.models && result.models.length > 0) {
      availableModels.value = result.models
    }
  } catch (err) {
    testResult.value = {
      ok: false,
      error: err instanceof Error ? err.message : 'Connection test failed',
    }
  } finally {
    testing.value = false
  }
}

async function save(): Promise<void> {
  saving.value = true
  try {
    await saveAIConfig({ ...form })
    toasts.success('AI settings saved successfully')
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed to save settings')
  } finally {
    saving.value = false
  }
}

// ─── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(async () => {
  try {
    const cfg = await getAIConfig()
    form.enabled = cfg.enabled
    form.provider = cfg.provider || 'openai'
    form.base_url = cfg.base_url || PROVIDERS[form.provider]?.defaultUrl || ''
    form.api_key = cfg.api_key || ''
    form.model = cfg.model || ''
    form.max_tokens = cfg.max_tokens || 2048
    form.timeout_secs = cfg.timeout_secs || 30
    form.system_prompt = cfg.system_prompt || ''
  } catch {
    toasts.error('Failed to load AI configuration')
  } finally {
    loading.value = false
  }
})
</script>
