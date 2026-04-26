<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="max-w-lg">
    <h1 class="text-2xl font-bold text-foreground mb-6">{{ t('settings.profile') }}</h1>
    <div class="bg-surface rounded-lg border border-border p-6 mb-6">
      <p class="text-sm text-muted">{{ t('auth.username') }}</p>
      <p class="font-medium text-foreground">{{ session.user?.username }}</p>
      <p v-if="session.user?.email" class="mt-2 text-sm text-muted">Email</p>
      <p v-if="session.user?.email" class="font-medium text-foreground">{{ session.user.email }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useSessionStore } from '../stores/session'
import { t } from '../i18n/en'

const session = useSessionStore()
</script>
