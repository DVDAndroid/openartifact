// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, expect, it } from 'bun:test'
import { renderMarkdown } from '../markdown'

describe('renderMarkdown', () => {
  it('never lets raw HTML through', () => {
    const html = renderMarkdown('<script>alert(1)</script> <img src=x onerror=alert(1)>')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('<img')
    expect(html).toContain('&lt;script&gt;')
  })

  it('only links http and https', () => {
    expect(renderMarkdown('[ok](https://example.com/a?b=1&c=2)')).toContain('href="https://example.com/a?b=1&amp;c=2"')
    expect(renderMarkdown('[bad](javascript:alert(1))')).not.toContain('<a ')
  })

  it('cannot break out of an attribute', () => {
    const html = renderMarkdown('[x](https://e.com/"onmouseover="alert(1))')
    expect(html).not.toContain('onmouseover="alert')
  })

  it('renders headings, emphasis, code and lists', () => {
    const html = renderMarkdown('# Title\n\n**bold** and *it* and `code`\n\n- a\n- b\n\n1. one')
    expect(html).toContain('<h3>Title</h3>')
    expect(html).toContain('<strong>bold</strong>')
    expect(html).toContain('<em>it</em>')
    expect(html).toContain('<code>code</code>')
    expect(html).toContain('<ul>')
    expect(html).toContain('<li>b</li>')
    expect(html).toContain('<ol>')
  })

  it('keeps fenced code literal', () => {
    const html = renderMarkdown('```\n<b>**x**</b>\n```')
    expect(html).toContain('<pre><code>&lt;b&gt;**x**&lt;/b&gt;</code></pre>')
  })

  it('renders nothing for an empty text', () => {
    expect(renderMarkdown('')).toBe('')
  })
})
