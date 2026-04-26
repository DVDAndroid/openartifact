// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, it, expect } from 'bun:test'
import { mount } from '@vue/test-utils'
import DataTable from '../DataTable.vue'

const columns = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'count', label: 'Count' },
]

const rows = [
  { id: 1, name: 'a', count: 10 },
  { id: 2, name: 'b', count: 20 },
]

describe('DataTable', () => {
  it('renders correct number of rows', () => {
    const wrapper = mount(DataTable, { props: { columns, rows } })
    expect(wrapper.findAll('tbody tr').length).toBe(2)
  })

  it('renders correct number of header columns', () => {
    const wrapper = mount(DataTable, { props: { columns, rows } })
    expect(wrapper.findAll('thead th').length).toBe(2)
  })

  it('emits sort event with column key when sortable header clicked', async () => {
    const wrapper = mount(DataTable, { props: { columns, rows } })
    const headers = wrapper.findAll('thead th')
    await headers[0].trigger('click')
    const emitted = wrapper.emitted('sort') as string[][]
    expect(emitted).toBeTruthy()
    expect(emitted[0]).toEqual(['name'])
  })

  it('does not emit sort when non-sortable header clicked', async () => {
    const wrapper = mount(DataTable, { props: { columns, rows } })
    const headers = wrapper.findAll('thead th')
    const beforeCount = (wrapper.emitted('sort') ?? []).length
    await headers[1].trigger('click')
    const afterCount = (wrapper.emitted('sort') ?? []).length
    expect(afterCount).toBe(beforeCount)
  })
})
