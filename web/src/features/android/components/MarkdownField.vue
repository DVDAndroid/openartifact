<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- Markdown text shown rendered; with edit rights it switches to a textarea with a live preview. -->
<template>
  <section class="rounded-xl border border-border bg-surface">
    <header class="flex items-center justify-between gap-2 px-4 py-2.5 border-b border-border">
      <h3 class="text-sm font-semibold text-foreground">{{ title }}</h3>
      <AppButton v-if="editable && !editing" variant="ghost" size="xs" @click="startEdit">
        <PencilSquareIcon class="w-3.5 h-3.5" aria-hidden="true" />
        {{ t('common.edit') }}
      </AppButton>
    </header>

    <div v-if="!editing" class="px-4 py-3">
      <!-- renderMarkdown escapes the whole source and emits a fixed set of tags -->
      <div v-if="modelValue" class="markdown text-sm text-foreground" v-html="rendered" />
      <p v-else class="text-sm text-muted italic">{{ empty }}</p>
    </div>

    <div v-else class="p-4 grid gap-3 md:grid-cols-2">
      <textarea
        v-model="draft"
        rows="8"
        class="w-full rounded-lg border border-border-strong bg-surface p-2.5 text-sm font-mono text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
        :aria-label="title"
      />
      <div class="markdown rounded-lg border border-border p-3 text-sm text-foreground min-h-24" v-html="preview" />
      <div class="md:col-span-2 flex justify-end gap-2">
        <AppButton variant="secondary" size="sm" :disabled="busy" @click="editing = false">{{ t('common.cancel') }}</AppButton>
        <AppButton size="sm" :loading="busy" @click="save">{{ t('common.save') }}</AppButton>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { PencilSquareIcon } from '@heroicons/vue/24/outline'
import AppButton from '../../../components/AppButton.vue'
import { t } from '../../../i18n/en'
import { renderMarkdown } from '../markdown'

const props = defineProps<{
  title: string
  modelValue: string
  empty: string
  editable?: boolean
  busy?: boolean
}>()
const emit = defineEmits<{ save: [value: string] }>()

const editing = ref(false)
const draft = ref('')

const rendered = computed(() => renderMarkdown(props.modelValue))
const preview = computed(() => renderMarkdown(draft.value))

function startEdit(): void {
  draft.value = props.modelValue
  editing.value = true
}

function save(): void {
  emit('save', draft.value)
  editing.value = false
}
</script>

<style scoped>
.markdown :deep(p) { margin: 0 0 0.5rem; }
.markdown :deep(h3), .markdown :deep(h4), .markdown :deep(h5), .markdown :deep(h6) { font-weight: 600; margin: 0.75rem 0 0.25rem; }
.markdown :deep(ul) { list-style: disc; padding-left: 1.25rem; margin: 0 0 0.5rem; }
.markdown :deep(ol) { list-style: decimal; padding-left: 1.25rem; margin: 0 0 0.5rem; }
.markdown :deep(code) { font-family: ui-monospace, monospace; font-size: 0.85em; background: rgb(var(--color-surface-hover)); border-radius: 0.25rem; padding: 0.05rem 0.3rem; }
.markdown :deep(pre) { overflow-x: auto; background: rgb(var(--color-surface-hover)); border-radius: 0.5rem; padding: 0.6rem; margin: 0 0 0.5rem; }
.markdown :deep(pre code) { background: none; padding: 0; }
.markdown :deep(a) { color: rgb(var(--color-primary)); text-decoration: underline; }
</style>
