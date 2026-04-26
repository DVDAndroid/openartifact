// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { ErrorCodes } from '../api/error-codes'
import type { UserExtended as User } from '../api/generated/types.gen'

export type { User }

export const useSessionStore = defineStore('session', () => {
  const ready = ref(false)
  const user = ref<User | null>(null)

  const isAuthenticated = computed(() => user.value !== null)

  const mustChangePassword = computed(() => user.value?.must_change_password === true)

  const isAdmin = computed(() => {
    if (!user.value) return false
    if (user.value.is_admin === true) return true
    if (user.value.groups?.some((g) => g.name === 'admin')) return true
    return false
  })

  async function fetchMe(): Promise<void> {
    try {
      const res = await fetch('/api/v1/auth/me', {
        headers: { 'X-Requested-With': 'XMLHttpRequest' },
        credentials: 'include',
      })
      if (res.status === 401) {
        user.value = null
        return
      }
      if (res.status === 403) {
        // RBAC middleware blocks all routes when must_change_password is set,
        // including /auth/me. Treat that response as "authenticated but locked"
        // so the SPA can route the user to the change-password screen on reload.
        try {
          const body = (await res.json()) as { error?: string; code?: string }
          if (body?.code === ErrorCodes.PASSWORD_CHANGE_REQUIRED || body?.error === ErrorCodes.PASSWORD_CHANGE_REQUIRED) {
            user.value = { id: '', username: '', must_change_password: true }
            return
          }
        } catch {
          // fall through
        }
        user.value = null
        return
      }
      if (res.ok) {
        user.value = (await res.json()) as User
      }
    } catch {
      user.value = null
    } finally {
      ready.value = true
    }
  }

  function clearSession(): void {
    user.value = null
  }

  return { user, ready, isAuthenticated, isAdmin, mustChangePassword, fetchMe, clearSession }
})
