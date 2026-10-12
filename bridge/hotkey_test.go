package bridge

import "testing"

func TestParseHotkey(t *testing.T) {
	got, err := parseHotkey(" alt + ctrl + a ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Norm != "Ctrl+Alt+A" {
		t.Fatalf("norm = %q", got.Norm)
	}
	if got.VK != 0x41 {
		t.Fatalf("vk = %#x", got.VK)
	}
	if got.Mods&(modControl|modAlt|modNoRepeat) != (modControl | modAlt | modNoRepeat) {
		t.Fatalf("mods = %#x", got.Mods)
	}
	if got.Mods&modShift != 0 || got.Mods&modWin != 0 {
		t.Fatalf("unexpected mods %#x", got.Mods)
	}

	f12, err := parseHotkey("Ctrl+Shift+F12")
	if err != nil {
		t.Fatal(err)
	}
	if f12.Norm != "Ctrl+Shift+F12" || f12.VK != 0x7B {
		t.Fatalf("f12 = %+v", f12)
	}
	digit, err := parseHotkey("Win+1")
	if err != nil {
		t.Fatal(err)
	}
	if digit.Norm != "Win+1" || digit.VK != 0x31 {
		t.Fatalf("digit = %+v", digit)
	}
}

func TestParseHotkeyRejects(t *testing.T) {
	for _, raw := range []string{"A", "Ctrl+Alt", "Ctrl++A", "Ctrl+Alt+A+B", "Ctrl+Alt+Enter", ""} {
		if _, err := parseHotkey(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestReviewHotkeysConflict(t *testing.T) {
	_, _, note := reviewHotkeys("Ctrl+Alt+A", "alt+ctrl+a")
	if note != "两个快捷键冲突，请改成不同的组合" {
		t.Fatalf("note = %q", note)
	}
	left, right, note := reviewHotkeys("Ctrl+Alt+A", "Ctrl+Alt+T")
	if note != "" || left.Norm != "Ctrl+Alt+A" || right.Norm != "Ctrl+Alt+T" {
		t.Fatalf("defaults: %q %+v %+v", note, left, right)
	}
	left, right, note = reviewHotkeys("", "Ctrl+Alt+T")
	if note != "" || left.VK != 0 || right.Norm != "Ctrl+Alt+T" {
		t.Fatalf("empty shot: %q %+v %+v", note, left, right)
	}
	_, _, note = reviewHotkeys("A", "Ctrl+Alt+T")
	if note == "" || note == "两个快捷键冲突，请改成不同的组合" {
		t.Fatalf("invalid note = %q", note)
	}
}
