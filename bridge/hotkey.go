package bridge

import (
	"fmt"
	"strings"
)

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000
)

// parsedHotkey is a RegisterHotKey combo. Norm is the stable label, such as Ctrl+Alt+A.
type parsedHotkey struct {
	Mods uint32
	VK   uint32
	Norm string
}

func (h parsedHotkey) same(other parsedHotkey) bool {
	if h.VK == 0 || other.VK == 0 {
		return false
	}
	return h.VK == other.VK && (h.Mods&^modNoRepeat) == (other.Mods&^modNoRepeat)
}

func parseHotkey(raw string) (parsedHotkey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parsedHotkey{}, fmt.Errorf("快捷键是空的")
	}
	parts := strings.Split(raw, "+")
	var mods uint32
	var keyName string
	seenMod := false
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			return parsedHotkey{}, fmt.Errorf("格式应为 Ctrl+Alt+A")
		}
		switch strings.ToLower(token) {
		case "ctrl", "control":
			mods |= modControl
			seenMod = true
		case "alt":
			mods |= modAlt
			seenMod = true
		case "shift":
			mods |= modShift
			seenMod = true
		case "win", "meta", "super":
			mods |= modWin
			seenMod = true
		default:
			if keyName != "" {
				return parsedHotkey{}, fmt.Errorf("只能有一个主键")
			}
			keyName = token
		}
	}
	if !seenMod {
		return parsedHotkey{}, fmt.Errorf("至少要有一个修饰键（Ctrl、Alt、Shift 或 Win）")
	}
	if keyName == "" {
		return parsedHotkey{}, fmt.Errorf("还需要一个主键，例如 A 或 F1")
	}
	vk, label, ok := keyCode(keyName)
	if !ok {
		return parsedHotkey{}, fmt.Errorf("不认识的按键 %s", keyName)
	}
	return parsedHotkey{
		Mods: mods | modNoRepeat,
		VK:   vk,
		Norm: modifierLabel(mods) + "+" + label,
	}, nil
}

func modifierLabel(mods uint32) string {
	var parts []string
	if mods&modControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&modAlt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&modShift != 0 {
		parts = append(parts, "Shift")
	}
	if mods&modWin != 0 {
		parts = append(parts, "Win")
	}
	return strings.Join(parts, "+")
}

func keyCode(token string) (uint32, string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, "", false
	}
	upper := strings.ToUpper(token)
	if len(upper) == 1 {
		r := upper[0]
		if r >= 'A' && r <= 'Z' {
			return uint32(r), upper, true
		}
		if r >= '0' && r <= '9' {
			return uint32(r), upper, true
		}
	}
	if len(upper) >= 2 && upper[0] == 'F' {
		n := 0
		for _, r := range upper[1:] {
			if r < '0' || r > '9' {
				n = 0
				break
			}
			n = n*10 + int(r-'0')
		}
		if n >= 1 && n <= 12 {
			return 0x70 + uint32(n-1), fmt.Sprintf("F%d", n), true
		}
	}
	return 0, "", false
}

// reviewHotkeys parses both shortcuts. An empty shortcut means that hotkey is off.
// The returned note is empty when both shortcuts can be registered.
func reviewHotkeys(shot, translate string) (parsedHotkey, parsedHotkey, string) {
	var notes []string
	var shotKey, translateKey parsedHotkey
	if strings.TrimSpace(shot) != "" {
		parsed, err := parseHotkey(shot)
		if err != nil {
			notes = append(notes, "截图快捷键无效："+err.Error())
		} else {
			shotKey = parsed
		}
	}
	if strings.TrimSpace(translate) != "" {
		parsed, err := parseHotkey(translate)
		if err != nil {
			notes = append(notes, "截图翻译快捷键无效："+err.Error())
		} else {
			translateKey = parsed
		}
	}
	if shotKey.same(translateKey) {
		notes = append(notes, "两个快捷键冲突，请改成不同的组合")
	}
	return shotKey, translateKey, strings.Join(notes, "\n")
}
