<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!--
  One app of an android repository: its signing keys, notes and versions, and
  the full detail of the selected version. The version and the tab live in the
  query string (?v=<id>&tab=signature) so every view is linkable.
-->
<template>
  <div>
    <PageHeader :title="title">
      <template v-if="app" #meta>
        <FormatBadge format="android" />
        <code class="font-mono text-xs bg-surface-hover rounded-md px-2 py-0.5 border border-border text-muted">{{ app.app.package_name }}</code>
      </template>
      <template v-if="app" #actions>
        <AppButton variant="secondary" @click="copy(latestUrl)">
          <ClipboardDocumentIcon class="w-4 h-4 text-muted" aria-hidden="true" />
          {{ t('android.app.copyLatest') }}
        </AppButton>
        <AppButton v-if="repoInfo?.permissions?.write || session.isAdmin" variant="secondary" :to="`/repos/${encodeURIComponent(repoName)}?tab=signers`">
          <KeyIcon class="w-4 h-4 text-muted" aria-hidden="true" />
          {{ t('android.signers.title') }}
        </AppButton>
      </template>
    </PageHeader>

    <div v-if="loading" class="text-sm text-muted">{{ t('common.loading') }}</div>
    <div v-else-if="loadError" class="rounded-xl border border-danger/30 bg-surface p-8 text-center">
      <p class="text-sm text-danger">{{ loadError }}</p>
      <AppButton variant="secondary" size="sm" class="mt-3" @click="load">{{ t('browser.retry') }}</AppButton>
    </div>
    <EmptyState v-else-if="!app" :title="t('android.app.notFound')" :description="t('android.app.notFoundHint')" />

    <div v-else class="space-y-4">
      <section class="rounded-xl border border-border bg-surface p-4 flex flex-wrap items-start gap-4">
        <AppIcon :src="selected ? versionIconUrl(repoName, selected.id, 192) : undefined" :label="title" :size="72" />
        <div class="min-w-0 flex-1 space-y-2">
          <TagEditor :model-value="app.app.tags" :editable="canWrite" :suggestions="tagNames" @update:model-value="(tags) => saveApp({ tags })" />
          <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
            <span>{{ t('android.app.versionCount').replace('{n}', String(versions.length)) }}</span>
            <label v-if="canWrite" class="flex items-center gap-1.5">
              {{ t('android.app.policy') }}
              <select :value="app.app.signer_policy" class="rounded-md border border-border-strong bg-surface px-1.5 py-1 text-xs text-foreground" :aria-label="t('android.app.policy')" @change="(e) => saveApp({ signer_policy: (e.target as HTMLSelectElement).value as SignerPolicy })">
                <option v-for="p in SIGNER_POLICIES" :key="p" :value="p">{{ policyLabel(p) }}</option>
              </select>
            </label>
            <span v-else>{{ t('android.app.policy') }}: {{ policyLabel(app.app.signer_policy) }}</span>
          </div>
        </div>
      </section>

      <section class="rounded-xl border border-border bg-surface" :aria-label="t('android.app.keys')">
        <header class="px-4 py-2.5 border-b border-border">
          <h2 class="text-sm font-semibold text-foreground">{{ t('android.app.keys') }}</h2>
        </header>
        <ul>
          <li v-for="s in app.signers" :key="s.sha256" class="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2.5 border-t border-border first:border-t-0 text-xs">
            <SignerBadge :sha256="s.sha256" :trust="s.trust" :known="s.known" />
            <span class="text-muted">{{ s.versions === 1 ? t('android.app.keyVersion') : t('android.app.keyVersions').replace('{n}', String(s.versions)) }}</span>
            <span class="text-muted">{{ formatDate(s.first) }} → {{ formatDate(s.last) }}</span>
            <span class="flex gap-1"><StatusBadge v-for="o in s.origins" :key="o" :label="originLabel(o)" :variant="originVariant(o)" /></span>
            <span v-if="ownBuildWithForeignKey(s)" class="text-warning">{{ t('android.app.foreignKey') }}</span>
          </li>
        </ul>
      </section>

      <MarkdownField :title="t('android.v.notes')" :model-value="app.app.notes" :empty="t('android.app.notesEmpty')" :editable="canWrite" :busy="savingApp" @save="(notes) => saveApp({ notes })" />

      <section class="rounded-xl border border-border bg-surface overflow-hidden">
        <header class="px-4 py-2.5 border-b border-border">
          <h2 class="text-sm font-semibold text-foreground">{{ t('android.app.versions') }}</h2>
        </header>
        <div class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="border-b border-border bg-surface-hover text-xs font-semibold text-muted">
                <th class="text-left px-4 py-2.5">{{ t('android.v.version') }}</th>
                <th class="text-left px-4 py-2.5">{{ t('android.col.origin') }}</th>
                <th class="text-left px-4 py-2.5">{{ t('android.v.channel') }}</th>
                <th class="text-left px-4 py-2.5">{{ t('android.v.abis') }}</th>
                <th class="text-left px-4 py-2.5">SDK</th>
                <th class="text-left px-4 py-2.5">{{ t('android.col.signer') }}</th>
                <th class="text-right px-4 py-2.5">{{ t('android.col.size') }}</th>
                <th class="text-left px-4 py-2.5">{{ t('android.v.uploaded') }}</th>
                <th class="text-right px-4 py-2.5"><span class="sr-only">{{ t('common.actions') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="ver in versions"
                :key="ver.id"
                class="border-t border-border first:border-t-0 cursor-pointer transition-colors group"
                :class="ver.id === selected?.id ? 'bg-primary-soft/40' : 'hover:bg-surface-hover'"
                :aria-selected="ver.id === selected?.id"
                @click="select(ver.id)"
              >
                <td class="px-4 py-2.5 text-xs">
                  <button type="button" class="font-mono text-foreground font-medium text-left" @click.stop="select(ver.id)">{{ ver.version_name || '—' }}</button>
                  <span class="block font-mono text-muted">{{ ver.version_code }}<StarIcon v-if="ver.pinned" class="inline w-3 h-3 ml-1 text-warning" :title="t('android.prov.pinned')" /></span>
                </td>
                <td class="px-4 py-2.5"><StatusBadge :label="originLabel(ver.origin)" :variant="originVariant(ver.origin)" /></td>
                <td class="px-4 py-2.5 text-xs text-muted">{{ ver.channel || '—' }}</td>
                <td class="px-4 py-2.5 text-xs font-mono text-muted">{{ ver.abis.join(', ') || '—' }}</td>
                <td class="px-4 py-2.5 text-xs font-mono text-muted">{{ ver.min_sdk }}→{{ ver.target_sdk }}</td>
                <td class="px-4 py-2.5">
                  <SignerBadge v-if="ver.signer_sha256" :sha256="ver.signer_sha256" :trust="keyOf(ver.signer_sha256)?.trust" :known="!!keyOf(ver.signer_sha256)" />
                </td>
                <td class="px-4 py-2.5 text-right text-xs font-mono tabular-nums text-muted">{{ formatBytes(ver.size_bytes) }}</td>
                <td class="px-4 py-2.5 text-xs text-muted">{{ formatDate(ver.created_at) }}</td>
                <td class="px-4 py-2.5 text-right whitespace-nowrap">
                  <a :href="localPath(ver.download_url)" class="inline-flex w-7 h-7 items-center justify-center rounded border border-border text-muted hover:text-foreground hover:bg-surface-hover transition-colors" :title="`${t('repoDetail.download')} ${ver.version_name}`" :aria-label="`${t('repoDetail.download')} ${ver.version_name}`" @click.stop>
                    <ArrowDownTrayIcon class="w-3.5 h-3.5" aria-hidden="true" />
                  </a>
                  <button v-if="canDelete" type="button" class="ml-1 inline-flex w-7 h-7 items-center justify-center rounded border border-border text-danger hover:bg-surface-hover transition-colors" :title="`${t('common.delete')} ${ver.version_name}`" :aria-label="`${t('common.delete')} ${ver.version_name}`" @click.stop="pendingDelete = ver">
                    <TrashIcon class="w-3.5 h-3.5" aria-hidden="true" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <template v-if="selected">
        <div class="flex flex-wrap items-center gap-3">
          <h2 class="text-base font-semibold text-foreground">{{ selected.version_name }} <span class="font-mono text-sm text-muted">({{ selected.version_code }})</span></h2>
          <AppButton v-if="canWrite" variant="ghost" size="xs" @click="saveVersion({ pinned: !selected.pinned })">
            <StarIcon class="w-3.5 h-3.5" aria-hidden="true" />
            {{ selected.pinned ? t('android.app.unpin') : t('android.app.pin') }}
          </AppButton>
        </div>
        <SectionTabs :items="tabs" :active="activeTab" :label="t('android.app.versionTabs')" />

        <div v-if="detailLoading" class="text-sm text-muted">{{ t('common.loading') }}</div>
        <p v-else-if="detailError" role="alert" class="text-sm text-danger">{{ detailError }}</p>
        <template v-else-if="detail">
          <VersionOverview v-if="activeTab === 'overview'" :detail="detail" :can-write="canWrite" :busy="savingVersion" :tag-suggestions="tagNames" @meta="saveVersion" />
          <VersionIcons v-else-if="activeTab === 'icons'" :detail="detail" />
          <VersionManifest v-else-if="activeTab === 'manifest'" :repo="repoName" :detail="detail" />
          <VersionSignature v-else-if="activeTab === 'signature'" :repo="repoName" :detail="detail" :can-write="canWrite" @changed="reloadDetail" />
          <VersionNative v-else-if="activeTab === 'native'" :detail="detail" />
          <VersionDependencies v-else-if="activeTab === 'dependencies'" :detail="detail" />
          <VersionProvenance v-else-if="activeTab === 'provenance'" :repo="repoName" :detail="detail" :can-write="canWrite" :busy="savingVersion" :siblings="versions" @meta="saveVersion" @changed="reloadDetail" />
          <VersionEntries v-else :detail="detail" />
        </template>
      </template>
    </div>

    <ConfirmDialog
      :open="!!pendingDelete"
      :title="t('common.delete')"
      :description="t('android.app.confirmDelete').replace('{name}', pendingDelete ? `${pendingDelete.version_name} (${pendingDelete.version_code})` : '')"
      :confirm-label="t('common.delete')"
      :busy="deleting"
      @confirm="pendingDelete && removeVersion(pendingDelete)"
      @close="pendingDelete = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDownTrayIcon, ClipboardDocumentIcon, KeyIcon, StarIcon, TrashIcon } from '@heroicons/vue/24/outline'
import AppButton from '../../../components/AppButton.vue'
import ConfirmDialog from '../../../components/ConfirmDialog.vue'
import EmptyState from '../../../components/EmptyState.vue'
import FormatBadge from '../../../components/FormatBadge.vue'
import PageHeader from '../../../components/PageHeader.vue'
import SectionTabs, { type SectionTab } from '../../../components/SectionTabs.vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import { getRepository, type Repository } from '../../../api/repositories'
import { formatDate } from '../../../lib/datetime'
import { t } from '../../../i18n/en'
import { useSessionStore } from '../../../stores/session'
import { useToastStore } from '../../../stores/toast'
import { deleteVersion, getApp, getVersion, listSigners, listTags, listVersions, updateApp, updateVersion, versionIconUrl } from '../api'
import { formatBytes, originVariant } from '../lib'
import { originLabel, policyLabel } from '../labels'
import {
  SIGNER_POLICIES, type AndroidSigner, type AndroidVersion, type AppDetail, type AppMetaUpdate,
  type SignerPolicy, type SignerUse, type VersionDetail, type VersionMetaUpdate,
} from '../types'
import AppIcon from '../components/AppIcon.vue'
import MarkdownField from '../components/MarkdownField.vue'
import SignerBadge from '../components/SignerBadge.vue'
import TagEditor from '../components/TagEditor.vue'
import VersionDependencies from '../components/VersionDependencies.vue'
import VersionEntries from '../components/VersionEntries.vue'
import VersionIcons from '../components/VersionIcons.vue'
import VersionManifest from '../components/VersionManifest.vue'
import VersionNative from '../components/VersionNative.vue'
import VersionOverview from '../components/VersionOverview.vue'
import VersionProvenance from '../components/VersionProvenance.vue'
import VersionSignature from '../components/VersionSignature.vue'

const TAB_IDS = ['overview', 'icons', 'manifest', 'signature', 'native', 'dependencies', 'provenance', 'files'] as const
type TabId = (typeof TAB_IDS)[number]

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const toasts = useToastStore()

const repoName = computed(() => route.params.name as string)
const pkg = computed(() => route.params.pkg as string)

const repoInfo = ref<Repository | null>(null)
const app = ref<AppDetail | null>(null)
const versions = ref<AndroidVersion[]>([])
const signers = ref<AndroidSigner[]>([])
const tagNames = ref<string[]>([])
const loading = ref(true)
const loadError = ref('')
const savingApp = ref(false)
const savingVersion = ref(false)
const pendingDelete = ref<AndroidVersion | null>(null)
const deleting = ref(false)

const detail = ref<VersionDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')

const canWrite = computed(() => session.isAdmin || repoInfo.value?.permissions?.write === true)
const canDelete = computed(() => session.isAdmin || repoInfo.value?.permissions?.delete === true)

const title = computed(() => {
  const a = app.value?.app
  return a?.label_override || selected.value?.label || a?.package_name || pkg.value
})

const selected = computed<AndroidVersion | null>(() => {
  const wanted = typeof route.query.v === 'string' ? route.query.v : ''
  return versions.value.find((v) => v.id === wanted) ?? versions.value[0] ?? null
})

const activeTab = computed<TabId>(() => {
  const wanted = typeof route.query.tab === 'string' ? route.query.tab : ''
  return (TAB_IDS as readonly string[]).includes(wanted) ? (wanted as TabId) : 'overview'
})

const tabs = computed<SectionTab[]>(() =>
  TAB_IDS.map((id) => ({
    id,
    label: t(`android.tab.${id}`),
    to: { query: { ...route.query, v: selected.value?.id, tab: id === 'overview' ? undefined : id } },
  })),
)

const latestUrl = computed(() => `${window.location.origin}/repos/${encodeURIComponent(repoName.value)}/apps/${encodeURIComponent(pkg.value)}/latest.apk`)

function keyOf(sha: string): AndroidSigner | undefined {
  return signers.value.find((s) => s.sha256 === sha)
}

// A build you made (or patched) that is not signed with a key you marked as yours.
function ownBuildWithForeignKey(s: SignerUse): boolean {
  return s.origins.some((o) => o === 'built' || o === 'patched') && !(s.known && s.trust === 'mine')
}

// The server may be configured with an external base URL; the session cookie
// only travels to this origin, so downloads go through the path.
function localPath(url: string): string {
  try {
    const u = new URL(url, window.location.origin)
    return u.pathname + u.search
  } catch {
    return url
  }
}

async function copy(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
    toasts.success(t('android.copied'))
  } catch {
    toasts.error(t('android.copyFailed'))
  }
}

function select(id: string): void {
  void router.replace({ query: { ...route.query, v: id } })
}

let generation = 0

async function load(silent = false): Promise<void> {
  const mine = ++generation
  if (!silent) loading.value = true
  loadError.value = ''
  try {
    const [repoRes, appRes, versionRes] = await Promise.all([
      getRepository(repoName.value),
      getApp(repoName.value, pkg.value),
      listVersions(repoName.value, pkg.value, { limit: 200 }),
    ])
    if (mine !== generation) return
    repoInfo.value = repoRes
    app.value = appRes
    versions.value = versionRes.items ?? []
    // Badges and suggestions are decoration: failing to load them must not hide the app.
    const [keys, tags] = await Promise.allSettled([listSigners(repoName.value, pkg.value), listTags(repoName.value)])
    if (mine !== generation) return
    signers.value = keys.status === 'fulfilled' ? (keys.value ?? []) : []
    tagNames.value = tags.status === 'fulfilled' ? (tags.value ?? []).map((x) => x.tag) : []
  } catch (err) {
    if (mine !== generation) return
    app.value = null
    const notFound = (err as { status?: number }).status === 404
    loadError.value = notFound ? '' : err instanceof Error ? err.message : t('android.app.loadFailed')
  } finally {
    if (mine === generation) loading.value = false
  }
}

let detailGeneration = 0

async function loadDetail(): Promise<void> {
  const mine = ++detailGeneration
  const id = selected.value?.id
  if (!id) {
    detail.value = null
    return
  }
  detailLoading.value = detail.value?.version.id !== id
  detailError.value = ''
  try {
    const res = await getVersion(repoName.value, id)
    if (mine === detailGeneration) detail.value = res
  } catch (err) {
    if (mine !== detailGeneration) return
    detail.value = null
    detailError.value = err instanceof Error ? err.message : t('android.app.loadFailed')
  } finally {
    if (mine === detailGeneration) detailLoading.value = false
  }
}

async function reloadDetail(): Promise<void> {
  await Promise.all([loadDetail(), load(true)])
}

async function saveApp(update: AppMetaUpdate): Promise<void> {
  savingApp.value = true
  try {
    await updateApp(repoName.value, pkg.value, update)
    await load(true)
  } catch (err) {
    toasts.error(t('android.app.saveFailed'), err instanceof Error ? err.message : undefined)
  } finally {
    savingApp.value = false
  }
}

async function saveVersion(update: VersionMetaUpdate): Promise<void> {
  if (!selected.value) return
  savingVersion.value = true
  try {
    await updateVersion(repoName.value, selected.value.id, update)
    await reloadDetail()
  } catch (err) {
    toasts.error(t('android.app.saveFailed'), err instanceof Error ? err.message : undefined)
  } finally {
    savingVersion.value = false
  }
}

async function removeVersion(ver: AndroidVersion): Promise<void> {
  deleting.value = true
  try {
    await deleteVersion(repoName.value, ver.id)
    pendingDelete.value = null
    toasts.success(t('android.app.deleted'))
    if (versions.value.length <= 1) {
      await router.push(`/repos/${encodeURIComponent(repoName.value)}`)
      return
    }
    await router.replace({ query: { ...route.query, v: undefined } })
    await load(true)
  } catch (err) {
    toasts.error(t('android.app.deleteFailed'), err instanceof Error ? err.message : undefined)
  } finally {
    deleting.value = false
  }
}

watch([repoName, pkg], () => void load(), { immediate: true })
watch(() => selected.value?.id, () => void loadDetail(), { immediate: true })
</script>
