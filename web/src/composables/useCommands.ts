// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { reactive, readonly } from 'vue'

export type CommandGroup = 'commands' | 'packages' | 'repositories'

export interface CommandEntry {
  id: string
  title: string
  hint?: string
  group: CommandGroup
  run: () => void | Promise<void>
}

const registry = reactive<Map<string, CommandEntry>>(new Map())

export function registerCommand(cmd: CommandEntry): () => void {
  registry.set(cmd.id, cmd)
  return () => registry.delete(cmd.id)
}

export function unregisterCommand(id: string): void {
  registry.delete(id)
}

export function listCommands(): CommandEntry[] {
  return Array.from(registry.values())
}

export function useCommands() {
  return {
    registry: readonly(registry),
    register: registerCommand,
    unregister: unregisterCommand,
    list: listCommands,
  }
}
