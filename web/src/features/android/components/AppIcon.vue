<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- App icon from the server, or the first letter of the label when the APK has no usable icon. -->
<template>
  <img
    v-if="src && !failed"
    :src="src"
    :alt="label"
    :width="size"
    :height="size"
    loading="lazy"
    class="shrink-0 rounded-lg bg-surface-hover object-contain"
    :style="{ width: `${size}px`, height: `${size}px` }"
    @error="failed = true"
  />
  <span
    v-else
    class="shrink-0 rounded-lg bg-surface-hover text-muted font-semibold inline-flex items-center justify-center select-none"
    :style="{ width: `${size}px`, height: `${size}px`, fontSize: `${Math.round(size * 0.45)}px` }"
    aria-hidden="true"
  >{{ initial }}</span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = withDefaults(defineProps<{ src?: string; label: string; size?: number }>(), { src: undefined, size: 40 })

const failed = ref(false)
watch(() => props.src, () => (failed.value = false))

const initial = computed(() => (props.label.trim()[0] ?? '?').toUpperCase())
</script>
