<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

<template>
  <div class="flex flex-col items-center justify-center min-h-[200px] gap-3 text-center p-6">
    <p class="text-sm text-danger font-medium">Failed to load page</p>
    <button
      class="text-xs text-primary hover:underline"
      @click="$router.go(0)"
    >
      Retry
    </button>
  </div>
</template>
