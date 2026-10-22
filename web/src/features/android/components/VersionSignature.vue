<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="space-y-4">
    <SignatureCard :version="v" :signer="detail.signer" :parent="detail.parent" />

    <section v-if="canWrite && v.signer_sha256" class="rounded-xl border border-border bg-surface p-4">
      <h3 class="text-sm font-semibold text-foreground mb-1">{{ t('android.sig.markTitle') }}</h3>
      <p class="text-xs text-muted mb-3">{{ t('android.sig.markHint') }}</p>
      <div class="flex flex-wrap gap-2">
        <AppButton v-for="tr in MARKS" :key="tr" size="sm" :variant="detail.signer?.trust === tr ? 'primary' : 'secondary'" :loading="marking === tr" @click="mark(tr)">
          {{ trustLabel(tr) }}
        </AppButton>
      </div>
    </section>

    <section v-for="r in v.signature?.results ?? []" :key="r.scheme" class="rounded-xl border border-border bg-surface p-4 space-y-3">
      <div class="flex items-center gap-2">
        <h3 class="text-sm font-semibold text-foreground">{{ t('android.sig.scheme') }} {{ r.scheme }}</h3>
        <StatusBadge :label="r.verified ? t('android.sig.verified') : t('android.sig.failed')" :variant="r.verified ? 'success' : 'danger'" />
      </div>
      <ul v-if="r.errors?.length || r.warnings?.length" class="list-disc pl-5 text-xs space-y-0.5">
        <li v-for="e in r.errors ?? []" :key="e" class="text-danger">{{ e }}</li>
        <li v-for="w in r.warnings ?? []" :key="w" class="text-warning">{{ w }}</li>
      </ul>
      <div v-for="(s, i) in r.signers" :key="i" class="rounded-lg border border-border bg-bg p-3 space-y-2">
        <p v-if="s.min_sdk || s.max_sdk" class="text-xs text-muted">SDK {{ s.min_sdk || 1 }}–{{ s.max_sdk || '∞' }}</p>
        <p v-if="s.digest_algorithms?.length" class="text-xs text-muted">{{ t('android.sig.digests') }}: <span class="font-mono text-foreground">{{ s.digest_algorithms.join(', ') }}</span></p>
        <p v-if="s.signature_algorithms?.length" class="text-xs text-muted">{{ t('android.sig.algorithms') }}: <span class="font-mono text-foreground">{{ s.signature_algorithms.join(', ') }}</span></p>
        <CertDetails v-if="s.certificate" :cert="s.certificate" />
        <ul v-if="s.errors?.length" class="list-disc pl-5 text-xs text-danger"><li v-for="e in s.errors" :key="e">{{ e }}</li></ul>
      </div>
      <div v-if="r.lineage?.length">
        <h4 class="text-xs font-semibold text-muted mb-1.5">{{ t('android.sig.lineage') }}</h4>
        <ol class="space-y-1.5">
          <li v-for="(n, i) in r.lineage" :key="i" class="text-xs font-mono text-foreground">
            {{ i + 1 }}. {{ n.certificate ? colonFingerprint(n.certificate.sha256) : '?' }}
          </li>
        </ol>
      </div>
    </section>

    <p v-if="!(v.signature?.results ?? []).length" class="text-sm text-muted">{{ t('android.sig.none') }}</p>

    <section v-if="checks" class="rounded-xl border border-border bg-surface p-4">
      <h3 class="text-sm font-semibold text-foreground mb-2">{{ t('android.sig.zipChecks') }}</h3>
      <ul class="space-y-1">
        <li v-for="c in checkRows" :key="c.label" class="flex items-center gap-2 text-xs">
          <component :is="c.ok ? CheckCircleIcon : ExclamationTriangleIcon" class="w-4 h-4" :class="c.ok ? 'text-success' : 'text-warning'" aria-hidden="true" />
          <span class="text-foreground">{{ c.label }}</span>
        </li>
      </ul>
      <ul v-if="checks.problems?.length" class="mt-2 list-disc pl-5 text-xs text-warning">
        <li v-for="p in checks.problems" :key="p">{{ p }}</li>
      </ul>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { CheckCircleIcon, ExclamationTriangleIcon } from '@heroicons/vue/24/outline'
import AppButton from '../../../components/AppButton.vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import { t } from '../../../i18n/en'
import { useToastStore } from '../../../stores/toast'
import { createSigner } from '../api'
import { colonFingerprint } from '../lib'
import { trustLabel } from '../labels'
import type { Trust, VersionDetail } from '../types'
import CertDetails from './CertDetails.vue'
import SignatureCard from './SignatureCard.vue'

const props = defineProps<{ repo: string; detail: VersionDetail; canWrite: boolean }>()
const emit = defineEmits<{ changed: [] }>()

const MARKS: Trust[] = ['mine', 'original', 'third_party', 'untrusted']

const toasts = useToastStore()
const marking = ref<Trust | ''>('')
const v = computed(() => props.detail.version)
const checks = computed(() => v.value.zip_checks)

const checkRows = computed(() => {
  const c = checks.value
  if (!c) return []
  return [
    { ok: c.aligned_4, label: t('android.sig.check.aligned4') },
    { ok: c.native_aligned_16k || !v.value.abis.length, label: t('android.sig.check.align16k') },
    { ok: !c.native_compressed, label: t('android.sig.check.nativeStored') },
    { ok: c.resources_arsc_stored || v.value.target_sdk < 30, label: t('android.sig.check.arscStored') },
    { ok: !c.prefix_bytes, label: t('android.sig.check.noPrefix') },
    { ok: !c.duplicate_entries?.length, label: t('android.sig.check.noDuplicates') },
  ]
})

// "Mine" describes a key you own, so it applies to the whole repository; the
// other levels describe a key's role for one app.
async function mark(trust: Trust): Promise<void> {
  marking.value = trust
  try {
    await createSigner(props.repo, {
      sha256: v.value.signer_sha256,
      trust,
      app: trust === 'mine' ? undefined : v.value.package_name,
    })
    toasts.success(t('android.sig.marked'))
    emit('changed')
  } catch (err) {
    toasts.error(t('android.sig.markFailed'), err instanceof Error ? err.message : undefined)
  } finally {
    marking.value = ''
  }
}
</script>
