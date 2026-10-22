<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<!-- A signing certificate: subject, validity, key and the three fingerprints. -->
<template>
  <div class="space-y-2">
    <dl class="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1 text-xs">
      <dt class="text-muted">{{ t('android.cert.subject') }}</dt><dd class="font-mono text-foreground break-all">{{ cert.subject }}</dd>
      <dt class="text-muted">{{ t('android.cert.issuer') }}</dt>
      <dd class="font-mono text-foreground break-all">{{ cert.issuer }}<span v-if="cert.self_signed" class="ml-2 text-muted">({{ t('android.cert.selfSigned') }})</span></dd>
      <dt class="text-muted">{{ t('android.cert.serial') }}</dt><dd class="font-mono text-foreground break-all">{{ cert.serial }}</dd>
      <dt class="text-muted">{{ t('android.cert.valid') }}</dt>
      <dd class="text-foreground">
        {{ formatDate(cert.not_before) }} → {{ formatDate(cert.not_after) }}
        <span v-if="expired" class="ml-2 text-warning">{{ t('android.cert.expired') }}</span>
      </dd>
      <dt class="text-muted">{{ t('android.cert.key') }}</dt>
      <dd class="font-mono text-foreground">{{ cert.public_key_algorithm }} {{ cert.public_key_size }} · {{ cert.signature_algorithm }}</dd>
    </dl>
    <ul v-if="cert.flags?.length" class="space-y-0.5">
      <li v-for="f in cert.flags" :key="f" class="text-xs text-warning">{{ tDynamic(`android.flag.${f}`) }}</li>
    </ul>
    <div class="space-y-1">
      <CopyField label="SHA-256" :value="colonFingerprint(cert.sha256)" />
      <CopyField label="SHA-1" :value="colonFingerprint(cert.sha1)" />
      <CopyField label="MD5" :value="colonFingerprint(cert.md5)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { formatDate } from '../../../lib/datetime'
import { t, tDynamic } from '../../../i18n/en'
import { colonFingerprint } from '../lib'
import type { CertInfo } from '../types'
import CopyField from './CopyField.vue'

const props = defineProps<{ cert: CertInfo }>()

// Android does not check expiry when installing; this is informational only.
const expired = computed(() => new Date(props.cert.not_after).getTime() < Date.now())
</script>
