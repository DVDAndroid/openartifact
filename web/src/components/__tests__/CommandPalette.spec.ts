// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, it, expect, beforeEach, afterEach, mock } from 'bun:test'
import { setActivePinia, createPinia } from 'pinia'
import { useUIStore } from '../../stores/ui'
import { registerCommand, unregisterCommand, listCommands } from '../../composables/useCommands'

// We test the small surface this skill controls: store toggles and the
// command registry. The full Headless UI Dialog mounted via @vue/test-utils
// requires DOM features (transitions, portals) that are awkward to mock
// without dragging in @testing-library/vue, which is not part of the
// dependency baseline.

describe('CommandPalette wiring', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  // The command registry is module-level state shared by every test file.
  afterEach(() => {
    unregisterCommand('test.cmd')
    unregisterCommand('test.run')
  })

  it('UI store opens and closes the palette', () => {
    const ui = useUIStore()
    expect(ui.commandPaletteOpen).toBe(false)
    ui.openCommandPalette()
    expect(ui.commandPaletteOpen).toBe(true)
    ui.closeCommandPalette()
    expect(ui.commandPaletteOpen).toBe(false)
  })

  it('registerCommand returns an unregister function', () => {
    const run = mock()
    const off = registerCommand({
      id: 'test.cmd',
      title: 'Test command',
      group: 'commands',
      run,
    })
    expect(listCommands().some((c) => c.id === 'test.cmd')).toBe(true)
    off()
    expect(listCommands().some((c) => c.id === 'test.cmd')).toBe(false)
  })

  it('command run is invoked when called directly', async () => {
    const run = mock()
    registerCommand({ id: 'test.run', title: 'Run thing', group: 'commands', run })
    const cmd = listCommands().find((c) => c.id === 'test.run')!
    await cmd.run()
    expect(run).toHaveBeenCalled()
  })
})
