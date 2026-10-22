// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { request } from './client'
import type {
  Project,
  ProjectCreateRequest,
  ProjectGroup,
  ProjectGroupUpdateRequest,
  ProjectMember,
  ProjectMemberCreateRequest,
  ProjectMemberUpdateRequest,
  ProjectUpdateRequest,
} from './generated/types.gen'

export type {
  Project,
  ProjectCreateRequest,
  ProjectGroup,
  ProjectGroupUpdateRequest,
  ProjectMember,
  ProjectMemberCreateRequest,
  ProjectMemberUpdateRequest,
  ProjectUpdateRequest,
}

/** Name of the built-in system project: the default, whose membership grants nothing. */
export const SYSTEM_PROJECT_NAME = 'system'

/** Roles a user can hold in a project, highest first. */
export const PROJECT_MEMBER_ROLES = ['owner', 'maintainer', 'developer', 'viewer'] as const
export type ProjectMemberRole = (typeof PROJECT_MEMBER_ROLES)[number]

/** Roles a group can be attached with (groups never own a project). */
export const PROJECT_GROUP_ROLES = ['maintainer', 'developer', 'viewer'] as const
export type ProjectGroupRole = (typeof PROJECT_GROUP_ROLES)[number]

/** Repository types a project may allow. */
export const PROJECT_REPO_TYPES = ['hosted', 'proxy', 'group'] as const
export type ProjectRepoType = (typeof PROJECT_REPO_TYPES)[number]

const JSON_HEADERS = { 'Content-Type': 'application/json' }
const enc = encodeURIComponent

/** Projects the caller belongs to; `all` lists every project (global admins only, others get 403). */
export async function listProjects(opts: { all?: boolean } = {}): Promise<Project[]> {
  const res = await request<{ projects?: Project[] }>(`/api/v1/projects${opts.all ? '?all=true' : ''}`)
  return res?.projects ?? []
}

export const createProject = (body: ProjectCreateRequest) =>
  request<Project>('/api/v1/projects', { method: 'POST', headers: JSON_HEADERS, body: JSON.stringify(body) })

export const updateProject = (project: string, body: ProjectUpdateRequest) =>
  request<Project>(`/api/v1/projects/${enc(project)}`, {
    method: 'PUT',
    headers: JSON_HEADERS,
    body: JSON.stringify(body),
  })

export const deleteProject = (project: string) =>
  request<void>(`/api/v1/projects/${enc(project)}`, { method: 'DELETE' })

export async function listProjectMembers(project: string): Promise<ProjectMember[]> {
  const res = await request<{ members?: ProjectMember[] }>(`/api/v1/projects/${enc(project)}/members`)
  return res?.members ?? []
}

export const addProjectMember = (project: string, body: ProjectMemberCreateRequest) =>
  request<ProjectMember>(`/api/v1/projects/${enc(project)}/members`, {
    method: 'POST',
    headers: JSON_HEADERS,
    body: JSON.stringify(body),
  })

export const updateProjectMember = (project: string, userId: string, body: ProjectMemberUpdateRequest) =>
  request<ProjectMember>(`/api/v1/projects/${enc(project)}/members/${enc(userId)}`, {
    method: 'PUT',
    headers: JSON_HEADERS,
    body: JSON.stringify(body),
  })

export const removeProjectMember = (project: string, userId: string) =>
  request<void>(`/api/v1/projects/${enc(project)}/members/${enc(userId)}`, { method: 'DELETE' })

export async function listProjectGroups(project: string): Promise<ProjectGroup[]> {
  const res = await request<{ groups?: ProjectGroup[] }>(`/api/v1/projects/${enc(project)}/groups`)
  return res?.groups ?? []
}

/** Attaches a group to the project, or changes the role of an already attached group. */
export const setProjectGroup = (project: string, groupId: string, body: ProjectGroupUpdateRequest) =>
  request<ProjectGroup>(`/api/v1/projects/${enc(project)}/groups/${enc(groupId)}`, {
    method: 'PUT',
    headers: JSON_HEADERS,
    body: JSON.stringify(body),
  })

export const removeProjectGroup = (project: string, groupId: string) =>
  request<void>(`/api/v1/projects/${enc(project)}/groups/${enc(groupId)}`, { method: 'DELETE' })
