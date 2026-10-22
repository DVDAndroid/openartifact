<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-4">
    <SignatureCard :version="v" :signer="detail.signer" :parent="detail.parent" />

    <section class="rounded-xl border border-border bg-surface p-4 space-y-3">
      <dl class="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1.5 text-xs">
        <dt class="text-muted">{{ t('android.v.package') }}</dt>
        <dd class="font-mono text-foreground">{{ v.package_name }}</dd>
        <dt class="text-muted">{{ t('android.v.version') }}</dt>
        <dd class="font-mono text-foreground">{{ v.version_name }} ({{ v.version_code }})</dd>
        <dt class="text-muted">{{ t('android.v.sdk') }}</dt>
        <dd class="text-foreground">
          min {{ sdkLabel(v.min_sdk) }} · target {{ sdkLabel(v.target_sdk) }}<template v-if="v.compile_sdk"> · compile {{ v.compile_sdk }}</template>
        </dd>
        <dt class="text-muted">{{ t('android.v.abis') }}</dt>
        <dd class="font-mono text-foreground">{{ v.abis.length ? v.abis.join(', ') : t('android.v.noNative') }}</dd>
        <dt class="text-muted">{{ t('android.v.frameworks') }}</dt>
        <dd class="text-foreground">{{ v.frameworks?.length ? v.frameworks.join(', ') : '—' }}</dd>
        <dt class="text-muted">{{ t('android.v.size') }}</dt>
        <dd class="text-foreground">{{ formatBytes(v.size_bytes) }} · {{ v.file_kind }}</dd>
        <dt class="text-muted">{{ t('android.v.uploaded') }}</dt>
        <dd class="text-foreground">{{ formatDateTime(v.created_at) }}<template v-if="v.created_by"> · {{ v.created_by }}</template></dd>
        <dt class="text-muted">{{ t('android.v.downloads') }}</dt>
        <dd class="text-foreground">{{ v.download_count }}</dd>
        <dt class="text-muted">{{ t('android.v.origin') }}</dt>
        <dd><StatusBadge :label="originLabel(v.origin)" :variant="originVariant(v.origin)" /></dd>
        <dt class="text-muted">{{ t('android.v.channel') }}</dt>
        <dd class="text-foreground">{{ v.channel || '—' }}</dd>
        <template v-if="v.source_url">
          <dt class="text-muted">{{ t('android.v.sourceUrl') }}</dt>
          <dd class="min-w-0 break-all">
            <a v-if="isHttp(v.source_url)" :href="v.source_url" target="_blank" rel="noopener noreferrer" class="text-primary hover:underline">{{ v.source_url }}</a>
            <span v-else class="font-mono text-foreground">{{ v.source_url }}</span>
          </dd>
        </template>
      </dl>
      <CopyField :value="v.sha256" label="SHA-256" />
    </section>

    <section class="rounded-xl border border-border bg-surface p-4 space-y-2">
      <h3 class="text-sm font-semibold text-foreground">{{ t('android.v.install') }}</h3>
      <CopyField :value="v.download_url" :label="t('android.v.url')" />
      <CopyField :value="adbInstall(v.filename, v.file_kind)" label="adb" />
    </section>

    <section v-if="v.warnings?.length" class="rounded-xl border border-warning/40 bg-surface p-4">
      <h3 class="text-sm font-semibold text-foreground mb-2">{{ t('android.v.warnings') }}</h3>
      <ul class="list-disc pl-5 text-xs text-warning space-y-0.5">
        <li v-for="w in v.warnings" :key="w">{{ w }}</li>
      </ul>
    </section>

    <section class="rounded-xl border border-border bg-surface p-4">
      <h3 class="text-sm font-semibold text-foreground mb-2">{{ t('android.col.tags') }}</h3>
      <TagEditor :model-value="v.tags" :editable="canWrite" :suggestions="tagSuggestions" @update:model-value="(tags) => emit('meta', { tags })" />
    </section>

    <MarkdownField :title="t('android.v.changelog')" :model-value="v.changelog" :empty="t('android.v.changelogEmpty')" :editable="canWrite" :busy="busy" @save="(changelog) => emit('meta', { changelog })" />
    <MarkdownField :title="t('android.v.notes')" :model-value="v.notes" :empty="t('android.v.notesEmpty')" :editable="canWrite" :busy="busy" @save="(notes) => emit('meta', { notes })" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import { formatDateTime } from '../../../lib/datetime'
import { t } from '../../../i18n/en'
import { adbInstall, formatBytes, originVariant, sdkLabel } from '../lib'
import { originLabel } from '../labels'
import type { VersionDetail, VersionMetaUpdate } from '../types'
import CopyField from './CopyField.vue'
import MarkdownField from './MarkdownField.vue'
import SignatureCard from './SignatureCard.vue'
import TagEditor from './TagEditor.vue'

const props = defineProps<{ detail: VersionDetail; canWrite: boolean; busy?: boolean; tagSuggestions?: string[] }>()
const emit = defineEmits<{ meta: [update: VersionMetaUpdate] }>()

const v = computed(() => props.detail.version)

function isHttp(url: string): boolean {
  return /^https?:\/\//i.test(url)
}
</script>
