import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

test('frosted panels use the exact glass recipe on a dark gradient', () => {
  const css = readFileSync(new URL('../src/assets/styles/glass.css', import.meta.url), 'utf8')
  const recipe = [
    '--glass-bg: rgba(255, 255, 255, 0.08);',
    '--glass-bg-hover: rgba(255, 255, 255, 0.16);',
    '--glass-border: 1px solid rgba(255, 255, 255, 0.12);',
    '--glass-radius: 12px;',
    '--glass-shadow: 0 8px 32px rgba(0, 0, 0, 0.25);',
    '--glass-blur: blur(12px);',
    '--base-gradient: linear-gradient(165deg, #1a2436 0%, #101722 46%, #070b12 100%);',
  ]
  for (const line of recipe) {
    assert.ok(css.includes(line), `missing ${line}`)
  }
  assert.match(css, /body\s*\{[^}]*background:\s*var\(--base-gradient\)/)
  assert.match(css, /#app \.app-main\s*\{[^}]*background:\s*var\(--glass-bg\)/)
  assert.match(css, /#app \.gui-card,\s*\.lux-row\s*\{[^}]*border:\s*var\(--glass-border\)/)
  assert.match(css, /:hover\s*\{[^}]*background:\s*var\(--glass-bg-hover\)/)
})
