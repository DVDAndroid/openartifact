// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { defineStore } from 'pinia'
import { useStorage } from '@vueuse/core'
import { ref } from 'vue'

export const useUIStore = defineStore('ui', () => {
  const sidebarCollapsed = useStorage<boolean>('oa.sidebar.collapsed', false)
  const commandPaletteOpen = ref(false)

  function toggleSidebar(): void {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function openCommandPalette(): void {
    commandPaletteOpen.value = true
  }

  function closeCommandPalette(): void {
    commandPaletteOpen.value = false
  }

  return {
    sidebarCollapsed,
    commandPaletteOpen,
    toggleSidebar,
    openCommandPalette,
    closeCommandPalette,
  }
})
