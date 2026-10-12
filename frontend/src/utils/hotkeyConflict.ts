const modifierAlias: Record<string, string> = {
  ctrl: 'ctrl',
  control: 'ctrl',
  alt: 'alt',
  shift: 'shift',
  win: 'win',
  meta: 'win',
  super: 'win',
}

const describeHotkey = (raw: string) => {
  const trimmed = raw.trim()
  if (!trimmed) return { ok: true, norm: '', reason: '' }
  if (trimmed.split('+').some((part) => part.trim() === '')) {
    return { ok: false, norm: '', reason: '格式应为 Ctrl+Alt+A' }
  }
  let mods = 0
  let key = ''
  const labels: string[] = []
  for (const part of trimmed.split('+').map((item) => item.trim())) {
    const alias = modifierAlias[part.toLowerCase()]
    if (alias) {
      const bit = alias === 'ctrl' ? 1 : alias === 'alt' ? 2 : alias === 'shift' ? 4 : 8
      if ((mods & bit) === 0) {
        mods |= bit
        labels.push(alias === 'ctrl' ? 'Ctrl' : alias === 'alt' ? 'Alt' : alias === 'shift' ? 'Shift' : 'Win')
      }
      continue
    }
    if (key) return { ok: false, norm: '', reason: '只能有一个主键' }
    const label = keyLabel(part)
    if (!label) return { ok: false, norm: '', reason: `不认识的按键 ${part}` }
    key = label
  }
  if (!mods) return { ok: false, norm: '', reason: '至少要有一个修饰键（Ctrl、Alt、Shift 或 Win）' }
  if (!key) return { ok: false, norm: '', reason: '还需要一个主键，例如 A 或 F1' }
  const order = ['Ctrl', 'Alt', 'Shift', 'Win'].filter((item) => labels.includes(item))
  return { ok: true, norm: `${order.join('+')}+${key}`, reason: '' }
}

const keyLabel = (token: string) => {
  const upper = token.trim().toUpperCase()
  if (/^[A-Z0-9]$/.test(upper)) return upper
  const fn = /^F([1-9]|1[0-2])$/.exec(upper)
  if (fn) return `F${Number(fn[1])}`
  return ''
}

/** Immediate settings warning. Registration conflicts are reported by the Windows hotkey service. */
export const localHotkeyWarning = (shot: string, translate: string) => {
  const left = describeHotkey(shot)
  const right = describeHotkey(translate)
  const notes: string[] = []
  if (shot.trim() && !left.ok) notes.push(`截图快捷键无效：${left.reason}`)
  if (translate.trim() && !right.ok) notes.push(`截图翻译快捷键无效：${right.reason}`)
  if (left.ok && right.ok && left.norm && right.norm && left.norm === right.norm) {
    notes.push('两个快捷键冲突，请改成不同的组合')
  }
  return notes.join('\n')
}
