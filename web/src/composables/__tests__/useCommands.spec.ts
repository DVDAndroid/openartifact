// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, it, expect, beforeEach, afterEach } from 'bun:test'
import { useCommands, registerCommand, unregisterCommand, listCommands, type CommandEntry } from '../useCommands'

// The registry is module-level state shared by every test file in the process.
function clearRegistry(): void {
  const { registry } = useCommands()
  registry.forEach((_, id) => unregisterCommand(id))
}

describe('useCommands', () => {
  beforeEach(clearRegistry)
  afterEach(clearRegistry)

  it('registers a command and adds it to the registry', () => {
    const { registry, register } = useCommands()
    const cmd: CommandEntry = {
      id: 'test-cmd',
      title: 'Test Command',
      group: 'commands',
      run: () => {},
    }

    register(cmd)
    expect(registry.has('test-cmd')).toBe(true)
    expect(registry.get('test-cmd')).toEqual(cmd)
  })

  it('unregisters a command and removes it from the registry', () => {
    const { registry, register, unregister } = useCommands()
    const cmd: CommandEntry = {
      id: 'test-cmd',
      title: 'Test Command',
      group: 'commands',
      run: () => {},
    }

    register(cmd)
    expect(registry.has('test-cmd')).toBe(true)

    unregister('test-cmd')
    expect(registry.has('test-cmd')).toBe(false)
  })

  it('retrieves commands by id', () => {
    const { registry, register } = useCommands()
    const cmd: CommandEntry = {
      id: 'my-cmd',
      title: 'My Command',
      hint: 'This is a hint',
      group: 'packages',
      run: () => {},
    }

    register(cmd)
    const retrieved = registry.get('my-cmd')
    expect(retrieved).toEqual(cmd)
    expect(retrieved?.title).toBe('My Command')
    expect(retrieved?.hint).toBe('This is a hint')
  })

  it('returns an unregister function from register', () => {
    const { registry, register } = useCommands()
    const cmd: CommandEntry = {
      id: 'unregister-test',
      title: 'Unregister Test',
      group: 'repositories',
      run: () => {},
    }

    const unregister = register(cmd)
    expect(registry.has('unregister-test')).toBe(true)

    unregister()
    expect(registry.has('unregister-test')).toBe(false)
  })

  it('lists all registered commands', () => {
    const { register, list } = useCommands()
    const cmd1: CommandEntry = {
      id: 'cmd1',
      title: 'Command 1',
      group: 'commands',
      run: () => {},
    }
    const cmd2: CommandEntry = {
      id: 'cmd2',
      title: 'Command 2',
      group: 'packages',
      run: () => {},
    }

    register(cmd1)
    register(cmd2)

    const commands = list()
    expect(commands).toHaveLength(2)
    expect(commands).toContainEqual(cmd1)
    expect(commands).toContainEqual(cmd2)
  })

  it('handles multiple registrations and unregistrations', () => {
    const { register, unregister, list } = useCommands()
    const cmds: CommandEntry[] = [
      { id: 'a', title: 'A', group: 'commands', run: () => {} },
      { id: 'b', title: 'B', group: 'packages', run: () => {} },
      { id: 'c', title: 'C', group: 'repositories', run: () => {} },
    ]

    cmds.forEach((cmd) => register(cmd))
    expect(list()).toHaveLength(3)

    unregister('b')
    expect(list()).toHaveLength(2)
    expect(list().map((c) => c.id)).toEqual(['a', 'c'])
  })
})
