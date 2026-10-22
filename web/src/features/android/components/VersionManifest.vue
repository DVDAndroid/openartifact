<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-4">
    <p v-if="!m" class="text-sm text-muted">{{ t('android.manifest.none') }}</p>
    <template v-else>
      <section class="rounded-xl border border-border bg-surface p-4">
        <h3 class="text-sm font-semibold text-foreground mb-2">{{ t('android.manifest.application') }}</h3>
        <dl class="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1.5 text-xs">
          <dt class="text-muted">{{ t('android.v.package') }}</dt><dd class="font-mono text-foreground">{{ m.package }}</dd>
          <dt class="text-muted">{{ t('android.v.version') }}</dt><dd class="font-mono text-foreground">{{ m.version_name }} ({{ m.version_code }})</dd>
          <dt class="text-muted">minSdk / targetSdk</dt><dd class="text-foreground">{{ sdkLabel(m.min_sdk) }} / {{ sdkLabel(m.target_sdk) }}</dd>
          <dt class="text-muted">debuggable</dt>
          <dd :class="m.application.debuggable ? 'text-danger' : 'text-foreground'">{{ yesNo(m.application.debuggable) }}</dd>
          <dt class="text-muted">allowBackup</dt><dd class="text-foreground">{{ tri(m.application.allow_backup) }}</dd>
          <dt class="text-muted">usesCleartextTraffic</dt>
          <dd :class="m.application.uses_cleartext_traffic ? 'text-warning' : 'text-foreground'">{{ tri(m.application.uses_cleartext_traffic) }}</dd>
          <dt class="text-muted">networkSecurityConfig</dt><dd class="font-mono text-foreground">{{ m.application.network_security_config || '—' }}</dd>
          <dt v-if="m.shared_user_id" class="text-muted">sharedUserId</dt><dd v-if="m.shared_user_id" class="font-mono text-foreground">{{ m.shared_user_id }}</dd>
          <dt v-if="m.is_split" class="text-muted">split</dt><dd v-if="m.is_split" class="font-mono text-foreground">{{ m.split }}</dd>
        </dl>
      </section>

      <section class="rounded-xl border border-border bg-surface">
        <header class="flex flex-wrap items-center gap-2 px-4 py-2.5 border-b border-border">
          <h3 class="text-sm font-semibold text-foreground mr-auto">{{ t('android.manifest.permissions') }} <span class="text-muted font-normal">({{ permissions.length }})</span></h3>
          <input v-model="permFilter" type="search" class="h-8 w-48 rounded-lg border border-border-strong bg-surface px-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary" :placeholder="t('common.search')" :aria-label="t('common.search')" />
        </header>
        <p v-if="permissions.length === 0" class="px-4 py-3 text-sm text-muted">{{ t('android.manifest.noPermissions') }}</p>
        <div v-for="g in permissionGroups" :key="g.group" class="px-4 py-2.5 border-t border-border first:border-t-0">
          <h4 class="text-xs font-semibold mb-1.5" :class="g.group === 'dangerous' ? 'text-danger' : g.group === 'signature' ? 'text-warning' : 'text-muted'">
            {{ tDynamic(`android.perm.${g.group}`) }} ({{ g.items.length }})
          </h4>
          <ul class="flex flex-wrap gap-1.5">
            <li v-for="p in g.items" :key="p.name" class="rounded-md bg-surface-hover px-2 py-0.5 text-xs font-mono text-foreground" :title="p.name">
              {{ shortPermission(p.name) }}<span v-if="p.max_sdk" class="text-muted"> ≤{{ p.max_sdk }}</span>
            </li>
          </ul>
        </div>
      </section>

      <section v-if="features.length" class="rounded-xl border border-border bg-surface p-4">
        <h3 class="text-sm font-semibold text-foreground mb-2">{{ t('android.manifest.features') }}</h3>
        <ul class="flex flex-wrap gap-1.5">
          <li v-for="f in features" :key="f.name ?? f.gles_version" class="rounded-md bg-surface-hover px-2 py-0.5 text-xs font-mono text-foreground">
            {{ f.name || `OpenGL ES ${f.gles_version}` }}<span v-if="!f.required" class="text-muted"> ({{ t('android.manifest.optional') }})</span>
          </li>
        </ul>
      </section>

      <section class="rounded-xl border border-border bg-surface">
        <header class="px-4 py-2.5 border-b border-border">
          <h3 class="text-sm font-semibold text-foreground">{{ t('android.manifest.components') }} <span class="text-muted font-normal">({{ components.length }})</span></h3>
        </header>
        <div class="overflow-x-auto">
          <table class="w-full text-xs">
            <thead>
              <tr class="border-b border-border bg-surface-hover text-muted font-semibold">
                <th class="text-left px-4 py-2">{{ t('android.manifest.kind') }}</th>
                <th class="text-left px-4 py-2">{{ t('android.manifest.name') }}</th>
                <th class="text-left px-4 py-2">{{ t('android.manifest.exported') }}</th>
                <th class="text-left px-4 py-2">{{ t('android.manifest.permission') }}</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="c in components" :key="`${c.kind}:${c.name}`">
                <tr class="border-t border-border first:border-t-0 align-top">
                  <td class="px-4 py-2 text-muted">{{ c.kind }}<span v-if="c.launcher" class="ml-1 text-primary">· launcher</span></td>
                  <td class="px-4 py-2 font-mono text-foreground break-all">{{ c.name }}</td>
                  <td class="px-4 py-2" :class="c.exported && !c.permission ? 'text-warning' : 'text-foreground'">{{ c.exported === undefined ? '—' : yesNo(c.exported) }}</td>
                  <td class="px-4 py-2 font-mono text-muted break-all">{{ c.permission || '—' }}</td>
                </tr>
                <tr v-for="(f, i) in c.intent_filters ?? []" :key="`${c.name}:${i}`" class="bg-bg">
                  <td />
                  <td colspan="3" class="px-4 py-1.5 font-mono text-muted break-all">
                    <span v-for="a in f.actions ?? []" :key="a" class="mr-2">{{ a }}</span>
                    <span v-for="d in f.data ?? []" :key="d" class="mr-2 text-primary">{{ d }}</span>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="m.queries?.length" class="rounded-xl border border-border bg-surface p-4">
        <h3 class="text-sm font-semibold text-foreground mb-2">queries</h3>
        <ul class="flex flex-wrap gap-1.5">
          <li v-for="q in m.queries" :key="q" class="rounded-md bg-surface-hover px-2 py-0.5 text-xs font-mono text-foreground">{{ q }}</li>
        </ul>
      </section>
    </template>

    <section class="rounded-xl border border-border bg-surface">
      <header class="flex flex-wrap items-center gap-2 px-4 py-2.5 border-b border-border">
        <h3 class="text-sm font-semibold text-foreground mr-auto">AndroidManifest.xml</h3>
        <input v-if="xml !== null" v-model="xmlFilter" type="search" class="h-8 w-48 rounded-lg border border-border-strong bg-surface px-2.5 text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-primary" :placeholder="t('android.manifest.filterLines')" :aria-label="t('android.manifest.filterLines')" />
        <AppButton v-if="xml === null" variant="secondary" size="xs" :loading="xmlLoading" @click="loadXml">{{ t('android.manifest.showXml') }}</AppButton>
      </header>
      <p v-if="xmlError" role="alert" class="px-4 py-3 text-sm text-danger">{{ xmlError }}</p>
      <pre v-if="xml !== null" class="max-h-[32rem] overflow-auto px-4 py-3 text-xs font-mono text-foreground">{{ visibleXml }}</pre>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppButton from '../../../components/AppButton.vue'
import { t, tDynamic } from '../../../i18n/en'
import { getManifestXml } from '../api'
import { PERMISSION_GROUPS, permissionGroup, sdkLabel, shortPermission } from '../lib'
import type { VersionDetail } from '../types'

const props = defineProps<{ repo: string; detail: VersionDetail }>()

const m = computed(() => props.detail.version.manifest)
// The server encodes an empty list as null.
const permissions = computed(() => m.value?.permissions ?? [])
const features = computed(() => m.value?.features ?? [])
const components = computed(() => m.value?.components ?? [])
const permFilter = ref('')
const xml = ref<string | null>(null)
const xmlFilter = ref('')
const xmlLoading = ref(false)
const xmlError = ref('')

function yesNo(b: boolean): string {
  return b ? t('android.yes') : t('android.no')
}
function tri(b?: boolean): string {
  return b === undefined ? t('android.manifest.default') : yesNo(b)
}

const permissionGroups = computed(() => {
  const q = permFilter.value.trim().toLowerCase()
  const perms = permissions.value.filter((p) => !q || p.name.toLowerCase().includes(q))
  return PERMISSION_GROUPS
    .map((group) => ({ group, items: perms.filter((p) => permissionGroup(p) === group) }))
    .filter((g) => g.items.length > 0)
})

const visibleXml = computed(() => {
  const q = xmlFilter.value.trim().toLowerCase()
  if (!q || xml.value === null) return xml.value ?? ''
  return xml.value.split('\n').filter((l) => l.toLowerCase().includes(q)).join('\n')
})

async function loadXml(): Promise<void> {
  xmlLoading.value = true
  xmlError.value = ''
  try {
    xml.value = await getManifestXml(props.repo, props.detail.version.id)
  } catch (err) {
    xmlError.value = err instanceof Error ? err.message : t('android.manifest.xmlFailed')
  } finally {
    xmlLoading.value = false
  }
}

watch(() => props.detail.version.id, () => { xml.value = null; xmlFilter.value = ''; xmlError.value = '' })
</script>
