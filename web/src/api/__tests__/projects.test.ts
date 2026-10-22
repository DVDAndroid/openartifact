// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, it, expect, spyOn, afterEach } from 'bun:test'
import {
  listProjects,
  listProjectGroups,
  removeProjectGroup,
  setProjectGroup,
  updateProject,
  updateProjectMember,
} from '../projects'

const json = (body: unknown, status = 200): Response =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

let spy: ReturnType<typeof spyOn<typeof globalThis, 'fetch'>>
afterEach(() => spy?.mockRestore())

function lastCall(): { url: string; init: RequestInit } {
  const call = spy.mock.calls.at(-1)!
  return { url: String(call[0]), init: (call[1] ?? {}) as RequestInit }
}

describe('projects api', () => {
  it('lists the caller\'s projects, or every project with all=true', async () => {
    spy = spyOn(globalThis, 'fetch').mockImplementation(async () => json({ projects: [{ name: 'a' }] }))
    expect(await listProjects()).toEqual([{ name: 'a' }])
    expect(lastCall().url).toBe('/api/v1/projects')
    await listProjects({ all: true })
    expect(lastCall().url).toBe('/api/v1/projects?all=true')
  })

  it('sends a partial update with PUT and encodes the project name', async () => {
    spy = spyOn(globalThis, 'fetch').mockImplementation(async () => json({}))
    await updateProject('a/b', { max_repositories: 3, allowed_formats: [] })
    const { url, init } = lastCall()
    expect(url).toBe('/api/v1/projects/a%2Fb')
    expect(init.method).toBe('PUT')
    expect(JSON.parse(init.body as string)).toEqual({ max_repositories: 3, allowed_formats: [] })
  })

  it('updates a member role', async () => {
    spy = spyOn(globalThis, 'fetch').mockImplementation(async () => json({}))
    await updateProjectMember('p', 'u1', { role: 'maintainer' })
    const { url, init } = lastCall()
    expect(url).toBe('/api/v1/projects/p/members/u1')
    expect(init.method).toBe('PUT')
    expect(JSON.parse(init.body as string)).toEqual({ role: 'maintainer' })
  })

  it('attaches, lists and detaches groups', async () => {
    spy = spyOn(globalThis, 'fetch').mockImplementation(async () => json({ groups: [{ group_id: 'g1' }] }))
    await setProjectGroup('p', 'g1', { role: 'developer' })
    let call = lastCall()
    expect(call.url).toBe('/api/v1/projects/p/groups/g1')
    expect(call.init.method).toBe('PUT')
    expect(JSON.parse(call.init.body as string)).toEqual({ role: 'developer' })
    expect(await listProjectGroups('p')).toEqual([{ group_id: 'g1' }])
    expect(lastCall().url).toBe('/api/v1/projects/p/groups')
    spy.mockRestore()
    spy = spyOn(globalThis, 'fetch').mockImplementation(async () => new Response(null, { status: 204 }))
    await removeProjectGroup('p', 'g1')
    call = lastCall()
    expect(call.url).toBe('/api/v1/projects/p/groups/g1')
    expect(call.init.method).toBe('DELETE')
  })
})
