<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-5">
    <!-- Header card -->
    <div class="bg-surface rounded-xl border border-border p-6">
      <div class="text-lg font-bold tracking-tight">{{ t('settings.theme') }}</div>
      <div class="text-sm text-muted mt-0.5">{{ t('theme.description') }}</div>
    </div>

    <!-- Theme options card -->
    <div class="bg-surface rounded-xl border border-border overflow-hidden">
      <div class="px-5 py-3.5 border-b border-border">
        <h3 class="text-sm font-semibold tracking-tight">{{ t('theme.selectMode') }}</h3>
      </div>
      <div class="p-5">
        <div class="grid grid-cols-3 gap-3">
          <button
            v-for="opt in options"
            :key="opt.value"
            type="button"
            class="flex flex-col items-center gap-2 p-4 rounded-xl border text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:ring-primary"
            :class="
              theme.mode === opt.value
                ? 'border-primary bg-primary-soft text-primary'
                : 'border-border hover:bg-surface-hover text-foreground'
            "
            @click="theme.setMode(opt.value)"
          >
            <component :is="opt.icon" class="w-5 h-5" aria-hidden="true" />
            {{ opt.label }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'
import { SunIcon, MoonIcon, ComputerDesktopIcon } from '@heroicons/vue/24/outline'
import { useThemeStore, type ThemeMode } from '../../stores/theme'
import { t } from '../../i18n/en'

const theme = useThemeStore()

const options: { value: ThemeMode; label: string; icon: Component }[] = [
  { value: 'light', label: t('theme.light'), icon: SunIcon },
  { value: 'dark', label: t('theme.dark'), icon: MoonIcon },
  { value: 'auto', label: t('theme.auto'), icon: ComputerDesktopIcon },
]
</script>
