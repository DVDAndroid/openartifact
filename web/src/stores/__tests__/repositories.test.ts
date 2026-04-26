// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { beforeEach, describe, expect, it, spyOn, type Mock } from 'bun:test'
import { createPinia, setActivePinia } from 'pinia'
import * as repositoriesApi from '../../api/repositories'
import { useRepositoriesStore } from '../repositories'

// The API module is stubbed with spyOn() on its export, which the global
// afterEach (test/isolation.ts) reverts via mock.restore(). mock.module() would
// replace api/repositories for every test file that runs afterwards.
let listPackages: Mock<typeof repositoriesApi.listPackages>

describe('useRepositoriesStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listPackages = spyOn(repositoriesApi, 'listPackages')
  })

  it('keeps the newest package page when an older request resolves last', async () => {
    const pageOne = Promise.withResolvers<{ packages: { name: string; version: string; format: string }[]; total: number }>()
    const pageTwo = Promise.withResolvers<{ packages: { name: string; version: string; format: string }[]; total: number }>()
    listPackages.mockReturnValueOnce(pageOne.promise).mockReturnValueOnce(pageTwo.promise)
    const store = useRepositoriesStore()

    const first = store.fetchPackages('repo', { page: 1, limit: 25 })
    const second = store.fetchPackages('repo', { page: 2, limit: 25 })

    pageTwo.resolve({ packages: [{ name: 'page-two', version: '2.0.0', format: 'npm' }], total: 26 })
    await second
    expect(store.packages).toEqual([{ name: 'page-two', version: '2.0.0', format: 'npm' }])
    expect(store.total).toBe(26)
    expect(store.loading).toBe(false)

    pageOne.resolve({ packages: [{ name: 'page-one', version: '1.0.0', format: 'npm' }], total: 25 })
    await first
    expect(store.packages).toEqual([{ name: 'page-two', version: '2.0.0', format: 'npm' }])
    expect(store.total).toBe(26)
    expect(store.loading).toBe(false)
  })
})
