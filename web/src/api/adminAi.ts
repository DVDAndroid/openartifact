// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { request } from './client'

export interface AIConfig {
  enabled: boolean
  provider: string
  base_url: string
  api_key: string
  model: string
  max_tokens: number
  timeout_secs: number
  system_prompt: string
}

export interface TestResult {
  ok: boolean
  models?: string[]
  error?: string
}

export function getAIConfig(): Promise<AIConfig> {
  return request<AIConfig>('/api/v1/admin/ai/config')
}

export function saveAIConfig(cfg: AIConfig): Promise<AIConfig> {
  return request<AIConfig>('/api/v1/admin/ai/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(cfg),
  })
}

export function testAIConfig(cfg: AIConfig): Promise<TestResult> {
  return request<TestResult>('/api/v1/admin/ai/test', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(cfg),
  })
}

export function listAIModels(): Promise<TestResult> {
  return request<TestResult>('/api/v1/admin/ai/models')
}
