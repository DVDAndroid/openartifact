<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- Every icon extracted from the APK, an adaptive-icon preview, and the app title in each language. -->
<template>
  <div class="space-y-4">
    <section class="rounded-xl border border-border bg-surface p-4 space-y-3">
      <div class="flex flex-wrap items-center gap-3">
        <h3 class="text-sm font-semibold text-foreground mr-auto">{{ t('android.icons.title') }}</h3>
        <label class="text-xs text-muted flex items-center gap-1.5">
          {{ t('android.icons.previewSize') }}
          <select v-model.number="px" class="rounded-md border border-border-strong bg-surface px-1.5 py-1 text-xs text-foreground">
            <option v-for="s in SIZES" :key="s" :value="s">{{ s }} px</option>
          </select>
        </label>
        <label class="text-xs text-muted flex items-center gap-1.5">
          {{ t('android.icons.background') }}
          <select v-model="bg" class="rounded-md border border-border-strong bg-surface px-1.5 py-1 text-xs text-foreground">
            <option value="light">{{ t('theme.light') }}</option>
            <option value="dark">{{ t('theme.dark') }}</option>
            <option value="checker">{{ t('android.icons.checker') }}</option>
          </select>
        </label>
      </div>

      <p v-if="detail.icons.length === 0" class="text-sm text-muted">{{ t('android.icons.none') }}</p>

      <div v-for="g in groups" :key="g.role" class="space-y-2">
        <h4 class="text-xs font-semibold text-muted">{{ tDynamic(`android.icons.role.${g.role}`) }}</h4>
        <ul class="flex flex-wrap gap-3">
          <li v-for="ic in g.icons" :key="ic.id" class="w-40 rounded-lg border border-border p-2 text-xs space-y-1">
            <div class="flex items-center justify-center rounded-md overflow-hidden" :class="bgClass" :style="{ height: `${Math.max(px, 48)}px` }">
              <img
                v-if="isRaster(ic) && ic.url"
                :src="ic.url"
                :alt="`${ic.role} ${ic.density}`"
                :width="px"
                :height="px"
                style="image-rendering: pixelated; object-fit: contain"
                :style="{ width: `${px}px`, height: `${px}px` }"
              />
              <span v-else-if="ic.color" class="rounded" :style="{ width: `${px}px`, height: `${px}px`, background: ic.color }" :title="ic.color" />
              <span v-else class="text-muted px-2 text-center">{{ t('android.icons.notRenderable') }}</span>
            </div>
            <p class="font-mono text-foreground">{{ ic.density || '—' }}<template v-if="ic.width"> · {{ ic.width }}×{{ ic.height }}</template></p>
            <p class="text-muted">{{ ic.mime.replace('image/', '') }} · {{ formatBytes(ic.size_bytes) }}</p>
            <p class="font-mono text-muted break-all" :title="ic.source_path">{{ ic.source_path }}</p>
            <div class="flex gap-1.5 pt-0.5">
              <a v-if="ic.url" :href="ic.url" :download="downloadName(ic)" class="text-primary hover:underline">{{ t('repoDetail.download') }}</a>
              <button v-if="ic.sha256" type="button" class="text-primary hover:underline" @click="copy(ic.sha256)">SHA-256</button>
            </div>
          </li>
        </ul>
      </div>
    </section>

    <section v-if="adaptive" class="rounded-xl border border-border bg-surface p-4 space-y-3">
      <div class="flex flex-wrap items-center gap-3">
        <h3 class="text-sm font-semibold text-foreground mr-auto">{{ t('android.icons.adaptive') }}</h3>
        <label class="text-xs text-muted flex items-center gap-1.5">
          {{ t('android.icons.mask') }}
          <select v-model="mask" class="rounded-md border border-border-strong bg-surface px-1.5 py-1 text-xs text-foreground">
            <option value="circle">{{ t('android.icons.mask.circle') }}</option>
            <option value="squircle">{{ t('android.icons.mask.squircle') }}</option>
            <option value="rounded">{{ t('android.icons.mask.rounded') }}</option>
            <option value="square">{{ t('android.icons.mask.square') }}</option>
          </select>
        </label>
      </div>
      <div class="flex flex-wrap gap-6">
        <figure v-for="variant in adaptiveVariants" :key="variant.key" class="text-center text-xs text-muted space-y-1">
          <div class="relative overflow-hidden" :class="bgClass" :style="{ width: `${px}px`, height: `${px}px`, borderRadius: maskRadius }">
            <img v-if="variant.background?.url" :src="variant.background.url" alt="" class="absolute inset-0 w-full h-full object-cover" />
            <span v-else-if="variant.background?.color" class="absolute inset-0" :style="{ background: variant.background.color }" />
            <img
              v-if="variant.foreground?.url"
              :src="variant.foreground.url"
              alt=""
              class="absolute inset-0 w-full h-full object-contain"
              style="transform: scale(1.5)"
            />
          </div>
          <figcaption>{{ variant.label }}</figcaption>
        </figure>
      </div>
    </section>

    <section class="rounded-xl border border-border bg-surface">
      <header class="flex flex-wrap items-center gap-2 px-4 py-2.5 border-b border-border">
        <h3 class="text-sm font-semibold text-foreground mr-auto">{{ t('android.icons.titles') }}</h3>
        <input v-model="titleFilter" type="search" class="h-8 w-48 rounded-lg border border-border-strong bg-surface px-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary" :placeholder="t('common.search')" :aria-label="t('common.search')" />
      </header>
      <p v-if="titles.length === 0" class="px-4 py-3 text-sm text-muted">{{ t('android.icons.noTitles') }}</p>
      <table v-else class="w-full text-xs">
        <thead>
          <tr class="border-b border-border bg-surface-hover text-muted font-semibold">
            <th class="text-left px-4 py-2">{{ t('android.icons.locale') }}</th>
            <th class="text-left px-4 py-2">{{ t('android.icons.titleCol') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in titles" :key="row.locale" class="border-t border-border first:border-t-0">
            <td class="px-4 py-2 font-mono text-muted">{{ row.locale || t('android.icons.default') }}</td>
            <td class="px-4 py-2 text-foreground">
              {{ row.title }}
              <span v-if="row.locale === ''" class="ml-1.5 text-muted">({{ t('android.icons.defaultTitle') }})</span>
              <span v-if="row.launcher" class="ml-1.5 text-muted">· {{ t('android.icons.launcherTitle') }}: {{ row.launcher }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { t, tDynamic } from '../../../i18n/en'
import { useToastStore } from '../../../stores/toast'
import { formatBytes } from '../lib'
import type { AndroidIcon, VersionDetail } from '../types'

const props = defineProps<{ detail: VersionDetail }>()

const SIZES = [48, 96, 192, 512]
const ROLE_ORDER = ['app', 'round', 'foreground', 'background', 'monochrome', 'other']

const toasts = useToastStore()
const px = ref(96)
const bg = ref<'light' | 'dark' | 'checker'>('light')
const mask = ref<'circle' | 'squircle' | 'rounded' | 'square'>('circle')
const titleFilter = ref('')

const bgClass = computed(() =>
  bg.value === 'dark' ? 'bg-[#111]' : bg.value === 'light' ? 'bg-white' : 'icon-checker',
)

const maskRadius = computed(() => ({ circle: '50%', squircle: '30%', rounded: '18%', square: '0' })[mask.value])

function isRaster(ic: AndroidIcon): boolean {
  return ic.mime === 'image/png' || ic.mime === 'image/webp' || ic.mime === 'image/jpeg' || ic.mime === 'image/gif'
}

const groups = computed(() => {
  const by = new Map<string, AndroidIcon[]>()
  for (const ic of props.detail.icons) {
    const role = ROLE_ORDER.includes(ic.role) ? ic.role : 'other'
    by.set(role, [...(by.get(role) ?? []), ic])
  }
  return ROLE_ORDER.filter((r) => by.has(r)).map((role) => ({ role, icons: by.get(role)! }))
})

// The biggest layer of each role makes the preview.
function biggest(role: string): AndroidIcon | undefined {
  return props.detail.icons
    .filter((i) => i.role === role && (i.url || i.color))
    .sort((a, b) => b.width - a.width)[0]
}

const adaptive = computed(() => !!biggest('foreground'))

const adaptiveVariants = computed(() => {
  const fg = biggest('foreground')
  const back = biggest('background')
  const mono = biggest('monochrome')
  const list = [{ key: 'full', label: t('android.icons.composite'), foreground: fg, background: back }]
  if (mono) list.push({ key: 'mono', label: t('android.icons.monochrome'), foreground: mono, background: undefined })
  return list
})

const titles = computed(() => {
  const v = props.detail.version
  const launcher = v.launcher_label && v.launcher_label !== v.label ? v.launcher_label : ''
  const labels = { ...(v.labels ?? {}) }
  if (!('' in labels) && v.label) labels[''] = v.label
  const q = titleFilter.value.trim().toLowerCase()
  return Object.entries(labels)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([locale, title]) => ({ locale, title, launcher: locale === '' ? launcher : '' }))
    .filter((r) => !q || r.locale.toLowerCase().includes(q) || r.title.toLowerCase().includes(q))
})

function downloadName(ic: AndroidIcon): string {
  const ext = ic.mime.split('/')[1] ?? 'png'
  return `${props.detail.version.package_name}-${ic.role}-${ic.density}.${ext}`
}

async function copy(value: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(value)
    toasts.success(t('android.copied'))
  } catch {
    toasts.error(t('android.copyFailed'))
  }
}
</script>

<style scoped>
.icon-checker {
  background-color: #fff;
  /* Transparency checkerboard as a tiled SVG: a flat pattern, not a gradient. */
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12'%3E%3Crect width='6' height='6' fill='%23d4d4d8'/%3E%3Crect x='6' y='6' width='6' height='6' fill='%23d4d4d8'/%3E%3C/svg%3E");
}
</style>
