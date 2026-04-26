<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="grid grid-cols-[200px_1fr] gap-6">
    <!-- Left Stepper -->
    <div class="space-y-1">
      <div
        v-for="(s, idx) in steps"
        :key="idx"
        class="flex items-center gap-3 py-2.5 px-3 rounded-lg transition-colors"
        :class="
          idx === step
            ? 'bg-primary-soft/40 text-primary'
            : idx < step
            ? 'text-foreground'
            : 'text-muted'
        "
      >
        <div
          class="w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold border-2 flex-shrink-0"
          :class="
            idx < step
              ? 'border-primary bg-primary text-white'
              : idx === step
              ? 'border-primary bg-primary text-white'
              : 'border-border bg-surface'
          "
        >
          <CheckIcon v-if="idx < step" class="w-4 h-4" />
          <span v-else>{{ idx + 1 }}</span>
        </div>
        <span class="text-sm font-medium">{{ s.label }}</span>
      </div>
    </div>

    <!-- Right Content -->
    <div class="bg-surface rounded-xl border border-border p-6 min-h-[420px] flex flex-col">
      <!-- Step Title & Subtitle -->
      <div>
        <h2 class="text-lg font-semibold">{{ steps[step].label }}</h2>
        <p class="text-sm text-muted mt-1">{{ stepSubtitles[step] }}</p>
      </div>

      <!-- Content Area -->
      <div class="flex-1 mt-5">
        <!-- Step 0: Identity -->
        <template v-if="step === 0">
          <div class="space-y-4">
            <!-- Name Input -->
            <div>
              <label for="repo-name" class="block text-sm font-medium text-foreground mb-2">
                Repository Name
              </label>
              <input
                id="repo-name"
                v-model="form.name"
                :disabled="isEdit"
                type="text"
                placeholder="my-repo"
                class="w-full px-3 py-2 border border-border rounded-md text-sm font-mono bg-bg text-foreground disabled:opacity-60 focus:outline-none focus:ring-2 focus:ring-primary"
              />
              <p class="text-xs text-muted mt-1">Lowercase letters, numbers, and hyphens only</p>
            </div>

            <!-- Visibility Toggle -->
            <div>
              <label class="block text-sm font-medium text-foreground mb-3">Visibility</label>
              <div class="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  class="p-3 rounded-lg border-2 transition-all text-center"
                  :class="
                    !form.public
                      ? 'border-primary bg-primary-soft/40'
                      : 'border-border bg-surface hover:border-border/60'
                  "
                  @click="form.public = false"
                >
                  <div class="text-sm font-medium">Private</div>
                  <div class="text-xs text-muted mt-1">Only you can access</div>
                </button>
                <button
                  type="button"
                  class="p-3 rounded-lg border-2 transition-all text-center"
                  :class="
                    form.public
                      ? 'border-primary bg-primary-soft/40'
                      : 'border-border bg-surface hover:border-border/60'
                  "
                  @click="form.public = true"
                >
                  <div class="text-sm font-medium">Public</div>
                  <div class="text-xs text-muted mt-1">Anyone can access</div>
                </button>
              </div>
            </div>
          </div>
        </template>

        <!-- Step 1: Format -->
        <template v-else-if="step === 1">
          <div class="grid grid-cols-3 gap-3">
            <button
              v-for="fmt in formats"
              :key="fmt"
              type="button"
              class="p-3 rounded-lg border-2 transition-all text-left flex items-center gap-3"
              :class="
                form.format === fmt
                  ? 'border-primary bg-primary-soft/40 ring-1 ring-primary/30'
                  : 'border-border bg-surface hover:border-border/60'
              "
              @click="form.format = fmt"
            >
              <div
                class="w-1 h-12 rounded-full flex-shrink-0"
                :style="{ backgroundColor: formatColor(fmt) }"
              />
              <span class="text-sm font-medium uppercase">{{ fmt }}</span>
            </button>
          </div>
        </template>

        <!-- Step 2: Type -->
        <template v-else-if="step === 2">
          <div class="grid grid-cols-1 gap-3">
            <button
              v-for="t in types"
              :key="t.value"
              type="button"
              class="p-4 rounded-lg border-2 transition-all text-left"
              :class="
                form.type === t.value
                  ? 'border-primary bg-primary-soft/40'
                  : 'border-border bg-surface hover:border-border/60'
              "
              @click="form.type = t.value"
            >
              <div class="flex items-start gap-3">
                <component :is="t.icon" class="w-5 h-5 text-primary flex-shrink-0 mt-0.5" />
                <div>
                  <div class="text-sm font-medium">{{ t.title }}</div>
                  <div class="text-xs text-muted mt-1">{{ t.description }}</div>
                </div>
              </div>
            </button>
          </div>
        </template>

        <!-- Step 3: Policies -->
        <template v-else-if="step === 3">
          <div class="space-y-4">
            <!-- Description -->
            <div>
              <label for="repo-desc" class="block text-sm font-medium text-foreground mb-2">
                Description
              </label>
              <textarea
                id="repo-desc"
                v-model="form.description"
                placeholder="Optional description of this repository"
                rows="4"
                class="w-full px-3 py-2 border border-border rounded-md text-sm bg-bg text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>

            <!-- Public Toggle -->
            <div class="flex items-center gap-3">
              <input
                id="repo-public"
                v-model="form.public"
                type="checkbox"
                class="rounded"
              />
              <label for="repo-public" class="text-sm font-medium text-foreground">
                Make this repository public
              </label>
            </div>
          </div>
        </template>

        <!-- Step 4: Review -->
        <template v-else-if="step === 4">
          <dl class="space-y-3 text-sm">
            <div class="flex justify-between py-2 border-b border-border/50">
              <dt class="text-muted">Name</dt>
              <dd class="text-foreground font-medium font-mono">{{ form.name }}</dd>
            </div>
            <div class="flex justify-between py-2 border-b border-border/50">
              <dt class="text-muted">Format</dt>
              <dd class="text-foreground uppercase">{{ form.format }}</dd>
            </div>
            <div class="flex justify-between py-2 border-b border-border/50">
              <dt class="text-muted">Type</dt>
              <dd class="text-foreground">{{ types.find(t => t.value === form.type)?.title }}</dd>
            </div>
            <div class="flex justify-between py-2 border-b border-border/50">
              <dt class="text-muted">Visibility</dt>
              <dd class="text-foreground">{{ form.public ? 'Public' : 'Private' }}</dd>
            </div>
            <div v-if="form.description" class="flex justify-between py-2">
              <dt class="text-muted">Description</dt>
              <dd class="text-foreground text-right max-w-xs">{{ form.description }}</dd>
            </div>
          </dl>
        </template>
      </div>

      <!-- Navigation -->
      <div class="flex items-center justify-between mt-5 pt-4 border-t border-border">
        <button
          v-if="step > 0"
          type="button"
          class="px-3 py-1.5 text-sm rounded-md border border-border text-foreground hover:bg-surface-hover disabled:opacity-50"
          @click="step--"
        >
          Back
        </button>
        <div v-else />
        <button
          v-if="step < steps.length - 1"
          type="button"
          class="px-3 py-1.5 text-sm rounded-md bg-primary text-primary-contrast hover:opacity-90"
          @click="step++"
        >
          Next
        </button>
        <button
          v-else
          type="button"
          :disabled="submitting"
          class="px-3 py-1.5 text-sm rounded-md bg-primary text-primary-contrast hover:opacity-90 disabled:opacity-50"
          @click="submit"
        >
          {{ submitting ? 'Creating...' : isEdit ? 'Update' : 'Create' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  TagIcon,
  CubeIcon,
  ServerIcon,
  ShieldCheckIcon,
  CheckCircleIcon,
  CheckIcon,
} from '@heroicons/vue/24/outline'
import { createRepository, getRepository, updateRepository } from '../api/repositories'
import { useToastStore } from '../stores/toast'
import { ALL_FORMATS, formatColor, type FormatId } from '../lib/formats'

const route = useRoute()
const router = useRouter()
const toasts = useToastStore()

const formats = ALL_FORMATS

const types = [
  {
    value: 'hosted',
    icon: ServerIcon,
    title: 'Hosted',
    description: 'Store artifacts directly',
  },
  {
    value: 'proxy',
    icon: ServerIcon,
    title: 'Proxy',
    description: 'Cache from upstream',
  },
  {
    value: 'group',
    icon: ServerIcon,
    title: 'Group',
    description: 'Aggregate multiple repos',
  },
]

const isEdit = computed(() => !!route.params.name)

const step = ref(0)
const submitting = ref(false)

const form = ref({
  name: '',
  format: 'docker' as FormatId,
  type: 'hosted',
  description: '',
  public: false,
})

const steps = [
  { label: 'Identity', icon: TagIcon },
  { label: 'Format', icon: CubeIcon },
  { label: 'Type', icon: ServerIcon },
  { label: 'Policies', icon: ShieldCheckIcon },
  { label: 'Review', icon: CheckCircleIcon },
]

const stepSubtitles = [
  'Choose a name and visibility for your repository',
  'Select the package format',
  'Choose the repository type',
  'Add optional description and policies',
  'Review your configuration',
]

onMounted(async () => {
  if (isEdit.value) {
    try {
      const repo = await getRepository(route.params.name as string)
      form.value = {
        name: repo.name,
        format: repo.format,
        type: 'hosted',
        description: repo.description || '',
        public: repo.public || false,
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to load repository'
      toasts.error(msg)
    }
  }
})

async function submit(): Promise<void> {
  submitting.value = true
  try {
    if (isEdit.value) {
      await updateRepository(form.value.name, {
        description: form.value.description,
        public: form.value.public,
      })
    } else {
      await createRepository({
        name: form.value.name,
        format: form.value.format,
        public: form.value.public,
      })
    }
    toasts.success(isEdit.value ? 'Repository updated' : 'Repository created')
    await router.push(`/repos/${form.value.name}`)
  } catch (err) {
    toasts.error(err instanceof Error ? err.message : 'Failed')
  } finally {
    submitting.value = false
  }
}
</script>
