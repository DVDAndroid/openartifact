<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <svg :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" class="inline-block">
    <polyline
      :points="points"
      fill="none"
      :stroke="color"
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  data: number[]
  width?: number
  height?: number
  trend?: 'up' | 'down'
}>(), {
  width: 64,
  height: 24,
  trend: 'up',
})

const color = computed(() => props.trend === 'up' ? '#22c55e' : '#ef4444')

const points = computed(() => {
  if (!props.data?.length) return ''
  const max = Math.max(...props.data)
  const min = Math.min(...props.data)
  const range = max - min || 1
  const stepX = props.width / (props.data.length - 1 || 1)
  return props.data
    .map((v, i) => `${(i * stepX).toFixed(1)},${(props.height - ((v - min) / range) * (props.height - 4) - 2).toFixed(1)}`)
    .join(' ')
})
</script>
