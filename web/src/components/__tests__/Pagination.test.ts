// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, it, expect } from 'bun:test'
import { mount } from '@vue/test-utils'
import Pagination from '../Pagination.vue'

describe('Pagination', () => {
  it('disables prev and enables next on first page', () => {
    const wrapper = mount(Pagination, { props: { total: 50, page: 1, limit: 25 } })
    const buttons = wrapper.findAll('button')
    const prev = buttons[0]
    const next = buttons[1]
    expect((prev.element as HTMLButtonElement).disabled).toBe(true)
    expect((next.element as HTMLButtonElement).disabled).toBe(false)
  })

  it('shows correct range text on first page', () => {
    const wrapper = mount(Pagination, { props: { total: 50, page: 1, limit: 25 } })
    expect(wrapper.text()).toContain('Showing 1–25 of 50')
  })

  it('emits change event with page 2 when next clicked', async () => {
    const wrapper = mount(Pagination, { props: { total: 50, page: 1, limit: 25 } })
    const buttons = wrapper.findAll('button')
    await buttons[1].trigger('click')
    const emitted = wrapper.emitted('change') as number[][]
    expect(emitted).toBeTruthy()
    expect(emitted[0]).toEqual([2])
  })

  it('disables next and enables prev on last page', () => {
    const wrapper = mount(Pagination, { props: { total: 50, page: 2, limit: 25 } })
    const buttons = wrapper.findAll('button')
    const prev = buttons[0]
    const next = buttons[1]
    expect((prev.element as HTMLButtonElement).disabled).toBe(false)
    expect((next.element as HTMLButtonElement).disabled).toBe(true)
  })
})
