// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, it, expect } from 'bun:test'
import { mount } from '@vue/test-utils'
import Sparkline from '../Sparkline.vue'

describe('Sparkline', () => {
  it('renders SVG element', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30, 40, 50] } })
    expect(wrapper.find('svg').exists()).toBe(true)
  })

  it('renders polyline with data', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30] } })
    expect(wrapper.find('polyline').exists()).toBe(true)
  })

  it('handles empty data', () => {
    const wrapper = mount(Sparkline, { props: { data: [] } })
    expect(wrapper.find('svg').exists()).toBe(true)
  })

  it('applies trend color up', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30], trend: 'up' } })
    const polyline = wrapper.find('polyline')
    expect(polyline.attributes('stroke')).toBe('#22c55e')
  })

  it('applies trend color down', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30], trend: 'down' } })
    const polyline = wrapper.find('polyline')
    expect(polyline.attributes('stroke')).toBe('#ef4444')
  })

  it('uses default trend up', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30] } })
    const polyline = wrapper.find('polyline')
    expect(polyline.attributes('stroke')).toBe('#22c55e')
  })

  it('renders with custom width', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30], width: 100 } })
    const svg = wrapper.find('svg')
    expect(svg.attributes('width')).toBe('100')
  })

  it('renders with custom height', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30], height: 50 } })
    const svg = wrapper.find('svg')
    expect(svg.attributes('height')).toBe('50')
  })

  it('uses default width 64', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30] } })
    const svg = wrapper.find('svg')
    expect(svg.attributes('width')).toBe('64')
  })

  it('uses default height 24', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30] } })
    const svg = wrapper.find('svg')
    expect(svg.attributes('height')).toBe('24')
  })

  it('polyline has correct attributes', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30] } })
    const polyline = wrapper.find('polyline')
    expect(polyline.attributes('fill')).toBe('none')
    expect(polyline.attributes('stroke-width')).toBe('1.5')
    expect(polyline.attributes('stroke-linecap')).toBe('round')
    expect(polyline.attributes('stroke-linejoin')).toBe('round')
  })

  it('renders single data point', () => {
    const wrapper = mount(Sparkline, { props: { data: [42] } })
    expect(wrapper.find('svg').exists()).toBe(true)
    expect(wrapper.find('polyline').exists()).toBe(true)
  })

  it('renders with increasing data', () => {
    const wrapper = mount(Sparkline, { props: { data: [1, 2, 3, 4, 5] } })
    const polyline = wrapper.find('polyline')
    expect(polyline.attributes('points')).toBeTruthy()
  })

  it('renders with decreasing data', () => {
    const wrapper = mount(Sparkline, { props: { data: [5, 4, 3, 2, 1] } })
    const polyline = wrapper.find('polyline')
    expect(polyline.attributes('points')).toBeTruthy()
  })

  it('renders with same values', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 10, 10, 10] } })
    const polyline = wrapper.find('polyline')
    expect(polyline.attributes('points')).toBeTruthy()
  })

  it('has inline-block class', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30] } })
    const svg = wrapper.find('svg')
    expect(svg.classes()).toContain('inline-block')
  })

  it('viewBox is set correctly', () => {
    const wrapper = mount(Sparkline, { props: { data: [10, 20, 30], width: 100, height: 50 } })
    const svg = wrapper.find('svg')
    expect(svg.attributes('viewBox')).toBe('0 0 100 50')
  })
})
