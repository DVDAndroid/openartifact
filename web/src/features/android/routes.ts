// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { RouteRecordRaw } from 'vue-router'
import type { CrumbSpec } from '../../lib/crumbs'

const trail: CrumbSpec[] = [
  { key: 'nav.repositories', to: '/repos' },
  { param: 'name', to: '/repos/:name' },
  { param: 'pkg' },
]

/**
 * The app page lives on the package URL shape the server already allows for
 * SPA navigation (`/repos/:name/packages/:format/:pkg`); a static `android`
 * segment ranks above the generic `:format`.
 */
export const androidRoutes: RouteRecordRaw[] = [
  {
    path: '/repos/:name/packages/android/:pkg',
    name: 'android-app',
    component: () => import('./pages/AndroidAppPage.vue'),
    meta: { crumbs: trail, requiresAuth: true },
  },
]
