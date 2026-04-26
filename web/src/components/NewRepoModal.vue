<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <!-- Backdrop -->
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in">
      <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="open = false" />
      <div class="relative w-full max-w-xl rounded-xl bg-surface border border-border shadow-2xl overflow-hidden animate-slide-up">
        <!-- Header -->
        <div class="flex items-center justify-between px-5 py-4 border-b border-border">
          <h3 class="text-base font-semibold tracking-tight">New repository</h3>
          <button @click="open = false" class="text-muted hover:text-foreground p-1 rounded hover:bg-surface-hover">
            <XMarkIcon class="w-4 h-4" />
          </button>
        </div>
        
        <!-- Body -->
        <div class="p-5 space-y-4">
          <!-- Name -->
          <div>
            <label class="text-[11px] uppercase tracking-[0.14em] font-semibold text-muted">Name</label>
            <input v-model="name" placeholder="e.g. docker-prod"
              class="mt-1.5 w-full h-10 px-3 rounded-md bg-surface-hover/40 border border-border focus:bg-surface focus:border-primary/50 focus:ring-2 focus:ring-primary/20 text-sm font-mono outline-none transition-colors" />
            <p class="mt-1 text-[11px] text-muted">Lowercase, hyphens, 2–64 characters.</p>
          </div>
          
          <!-- Format swatch picker -->
          <div>
            <label class="text-[11px] uppercase tracking-[0.14em] font-semibold text-muted">Format</label>
            <div class="mt-1.5 grid grid-cols-6 gap-1.5">
              <button v-for="f in formats" :key="f" @click="format = f"
                :class="['flex flex-col items-center gap-1 py-2 rounded-md border text-[11px] font-mono transition-all',
                  format === f ? 'border-primary bg-primary-soft text-primary' : 'border-border bg-surface hover:bg-surface-hover text-foreground/70']">
                <span class="w-2 h-2 rounded-sm" :style="{ background: formatColor(f) }" />
                {{ f }}
              </button>
            </div>
          </div>
          
          <!-- Type selector -->
          <div>
            <label class="text-[11px] uppercase tracking-[0.14em] font-semibold text-muted">Type</label>
            <div class="mt-1.5 grid grid-cols-3 gap-2">
              <button v-for="t in types" :key="t.value" @click="type = t.value"
                :class="['p-3 rounded-md border text-left transition-all',
                  type === t.value ? 'border-primary bg-primary-soft' : 'border-border bg-surface hover:bg-surface-hover']">
                <div :class="['text-sm font-semibold', type === t.value ? 'text-primary' : '']">{{ t.label }}</div>
                <div class="mt-0.5 text-[11px] text-muted leading-snug">{{ t.desc }}</div>
              </button>
            </div>
          </div>
          
          <!-- Public toggle -->
          <label class="flex items-center gap-2 pt-1 text-sm">
            <input type="checkbox" v-model="isPublic" class="accent-[rgb(var(--color-primary))]" />
            <span>Make repository public (anyone can pull)</span>
          </label>
        </div>
        
         <!-- Footer -->
         <div class="px-5 py-3 border-t border-border bg-surface-hover/30 flex items-center justify-end gap-2">
           <button @click="open = false" class="px-3 py-1.5 rounded-md text-sm text-muted hover:text-foreground hover:bg-surface-hover transition-colors">Cancel</button>
           <button @click="handleCreate" :disabled="!name.trim() || creating"
             class="px-4 py-1.5 rounded-md bg-primary text-primary-contrast text-sm font-medium hover:bg-primary/90 active:scale-[0.97] transition-all disabled:opacity-50 flex items-center gap-1.5">
             <CheckIcon class="w-4 h-4" />
             {{ creating ? 'Creating...' : 'Create repository' }}
           </button>
         </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { createRepository } from '../api/repositories'
import { useToastStore } from '../stores/toast'
import { XMarkIcon, CheckIcon } from '@heroicons/vue/24/outline'
import { ALL_FORMATS, formatColor, type FormatId } from '../lib/formats'

const open = defineModel<boolean>({ default: false })

const router = useRouter()
const toast = useToastStore()

const name = ref('')
const format = ref<FormatId>('docker')
const type = ref('hosted')
const isPublic = ref(false)
const creating = ref(false)

const formats = ALL_FORMATS

const types = [
  { value: 'hosted', label: 'Hosted', desc: 'Stores artifacts uploaded directly.' },
  { value: 'proxy', label: 'Proxy', desc: 'Read-through cache of an upstream repository.' },
  { value: 'group', label: 'Group', desc: 'Aggregates other repositories under one URL.' },
]

async function handleCreate() {
   if (!name.value.trim()) return
   creating.value = true
   try {
     await createRepository({
       name: name.value.trim(),
       format: format.value,
       public: isPublic.value,
       description: '',
     })
     toast.success('Repository created', `${name.value} is ready to use.`)
     open.value = false
     router.push(`/repos/${name.value.trim()}`)
   } catch (err: unknown) {
     toast.error('Failed to create repository', err instanceof Error ? err.message : 'Unknown error')
   } finally {
     creating.value = false
   }
 }

 // Handle Escape key
 watch(open, (newOpen) => {
   if (newOpen) {
     const handleKeydown = (e: KeyboardEvent) => {
       if (e.key === 'Escape') {
         open.value = false
       }
     }
     document.addEventListener('keydown', handleKeydown)
     return () => document.removeEventListener('keydown', handleKeydown)
   }
 })
</script>
