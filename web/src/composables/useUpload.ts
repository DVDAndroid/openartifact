// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { ref, computed } from 'vue'
import { ApiError, csrfHeader } from '../api/client'

export type UploadStatus = 'queued' | 'uploading' | 'complete' | 'failed' | 'cancelled'

export interface UploadEntry {
  id: number
  file: File
  status: UploadStatus
  loaded: number
  total: number
  error?: string
  xhr?: XMLHttpRequest
}

export interface UploadStartOptions {
  url: string
  field?: string
  headers?: Record<string, string>
}

let nextId = 1

export function useUpload() {
  const entries = ref<UploadEntry[]>([])

  const inProgress = computed(() => entries.value.some((e) => e.status === 'uploading' || e.status === 'queued'))

  function add(file: File): UploadEntry {
    const entry: UploadEntry = {
      id: nextId++,
      file,
      status: 'queued',
      loaded: 0,
      total: file.size,
    }
    entries.value.push(entry)
    return entry
  }

  function start(entry: UploadEntry, opts: UploadStartOptions): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      entry.xhr = xhr
      entry.status = 'uploading'
      xhr.open('POST', opts.url)
      xhr.withCredentials = true
      xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest')
      for (const [k, v] of Object.entries(csrfHeader())) xhr.setRequestHeader(k, v)
      for (const [k, v] of Object.entries(opts.headers ?? {})) xhr.setRequestHeader(k, v)
      xhr.upload.addEventListener('progress', (ev) => {
        if (ev.lengthComputable) {
          entry.loaded = ev.loaded
          entry.total = ev.total
        }
      })
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          entry.status = 'complete'
          entry.loaded = entry.total
          resolve()
        } else {
          entry.status = 'failed'
          entry.error = xhr.responseText || `HTTP ${xhr.status}`
          reject(new ApiError(entry.error, xhr.status))
        }
      })
      xhr.addEventListener('error', () => {
        entry.status = 'failed'
        entry.error = 'network error'
        reject(new ApiError('network error', 0))
      })
      xhr.addEventListener('abort', () => {
        entry.status = 'cancelled'
        reject(new ApiError('aborted', 0))
      })
      const form = new FormData()
      form.append(opts.field ?? 'file', entry.file, entry.file.name)
      xhr.send(form)
    })
  }

  function cancel(entry: UploadEntry): void {
    if (entry.xhr && (entry.status === 'uploading' || entry.status === 'queued')) {
      try {
        entry.xhr.abort()
      } catch {
        // ignore
      }
    }
    entry.status = 'cancelled'
  }

  function cancelAll(): void {
    for (const e of entries.value) cancel(e)
  }

  function clearCompleted(): void {
    entries.value = entries.value.filter((e) => e.status !== 'complete')
  }

  function reset(): void {
    cancelAll()
    entries.value = []
  }

  return { entries, inProgress, add, start, cancel, cancelAll, clearCompleted, reset }
}
