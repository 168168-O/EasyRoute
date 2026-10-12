import assert from 'node:assert/strict'
import test from 'node:test'

import { localHotkeyWarning } from '../src/utils/hotkeyConflict.ts'

test('screenshot hotkeys warn on conflict and bad format', () => {
  assert.equal(localHotkeyWarning('Ctrl+Alt+A', 'Ctrl+Alt+T'), '')
  assert.equal(localHotkeyWarning('alt+ctrl+a', 'Ctrl+Alt+A'), '两个快捷键冲突，请改成不同的组合')
  assert.match(localHotkeyWarning('A', 'Ctrl+Alt+T'), /截图快捷键无效/)
  assert.equal(localHotkeyWarning('', 'Ctrl+Alt+T'), '')
  assert.match(localHotkeyWarning('Ctrl+Alt+Enter', 'Ctrl+Alt+T'), /不认识的按键/)
})
