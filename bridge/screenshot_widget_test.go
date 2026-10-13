package bridge

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestDesktopWidgetChrome(t *testing.T) {
	text := desktopWidgetText("Ctrl+Alt+A", "Ctrl+Alt+T")
	for _, want := range []string{"截图", "截图翻译", "置顶", "一直浮在桌面最前面", "Ctrl+Alt+A", "Ctrl+Alt+T"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if containsRecentHistory(text) {
		t.Fatalf("history leaked into widget: %s", text)
	}
	custom := desktopWidgetText("Ctrl+Shift+F12", "")
	if !strings.Contains(custom, "Ctrl+Shift+F12") || !strings.Contains(custom, "Ctrl+Alt+T") {
		t.Fatalf("hotkey fallback = %q", custom)
	}
}

func TestTranslateResultChromeHasNoHistory(t *testing.T) {
	view := composeTranslateResult("Where is the station?", "车站在哪里？", "", "deepseek", true)
	if view.Original != "Where is the station?" || view.Chinese != "车站在哪里？" {
		t.Fatalf("body = %+v", view)
	}
	for _, want := range []string{"截图翻译", "截到的画面", "原文", "中文", "复制原文", "复制译文", "置顶", "来源：DeepSeek"} {
		if !strings.Contains(view.chromeText(), want) {
			t.Fatalf("missing %q in %q", want, view.chromeText())
		}
	}
	if containsRecentHistory(view.chromeText()) {
		t.Fatalf("history leaked into result chrome: %s", view.chromeText())
	}
	// Recognized text may contain those words. The window chrome still must not.
	noisy := composeTranslateResult("最近翻译", "最近的翻译", "", "", true)
	if containsRecentHistory(noisy.chromeText()) {
		t.Fatalf("chrome includes history labels: %s", noisy.chromeText())
	}
	if noisy.Original != "最近翻译" || noisy.Chinese != "最近的翻译" {
		t.Fatalf("body should keep the recognized text: %+v", noisy)
	}
	failed := composeTranslateResult("", "", "识别失败。请换一块更清晰的英文区域再试。", "", true)
	if failed.Chinese == "" || failed.ImageLabel != "截到的画面" {
		t.Fatalf("error result = %+v", failed)
	}
	if containsRecentHistory(failed.chromeText()) {
		t.Fatal(failed.chromeText())
	}
}

func TestFloatWidgetLayout(t *testing.T) {
	lay := layoutFloatWidget(floatWidgetW, floatWidgetH)
	if lay.Shot.overlaps(lay.Translate) || lay.Pin.overlaps(lay.Shot) || lay.Pin.overlaps(lay.Translate) {
		t.Fatalf("overlap %+v", lay)
	}
	if lay.Shot.W < 300 || lay.Translate.H < 80 {
		t.Fatalf("buttons too small %+v", lay)
	}
	if lay.Translate.Y+lay.Translate.H > floatWidgetH {
		t.Fatalf("translate button outside %+v", lay)
	}
	if !lay.Shot.contains(lay.Shot.X+4, lay.Shot.Y+4) || lay.Shot.contains(-1, -1) {
		t.Fatal("contains")
	}
}

func TestTranslateResultLayout(t *testing.T) {
	h := translateResultHeight(800, 400, 80, 60, 760)
	lay := layoutTranslateResult(translateResultW, h, 800, 400, 80, 60)
	if lay.Image.W <= 0 || lay.Image.H <= 0 {
		t.Fatalf("image %+v", lay)
	}
	if lay.CopyOriginal.overlaps(lay.Image) || lay.CopyChinese.overlaps(lay.Image) {
		t.Fatalf("buttons cover image %+v", lay)
	}
	if lay.CopyOriginal.overlaps(lay.Chinese) || lay.Original.overlaps(lay.Chinese) {
		t.Fatalf("text overlap %+v", lay)
	}
	if lay.Chinese.Y < lay.Original.Y+lay.Original.H {
		t.Fatalf("order %+v", lay)
	}
	if lay.CopyOriginal.Y < lay.Chinese.Y+lay.Chinese.H-1 {
		t.Fatalf("buttons not under text %+v", lay)
	}
	if lay.CopyChinese.X+lay.CopyChinese.W > translateResultW || lay.Note.Y+lay.Note.H > h {
		t.Fatalf("outside %+v h=%d", lay, h)
	}
	plain := layoutTranslateResult(translateResultW, translateResultHeight(0, 0, 40, 40, 760), 0, 0, 40, 40)
	if plain.Image.W != 0 || plain.ImageLabel.H != 0 {
		t.Fatalf("unexpected image %+v", plain)
	}
	for _, clientH := range []int32{300, 420, 586, 760} {
		for _, img := range [][2]int32{{0, 0}, {800, 400}, {120, 40}} {
			lay := layoutTranslateResult(translateResultW, clientH, img[0], img[1], 200, 180)
			boxes := []pxBox{lay.Original, lay.Chinese, lay.CopyOriginal, lay.CopyChinese, lay.Note}
			if img[0] > 0 && lay.Image.H > 0 {
				boxes = append(boxes, lay.Image)
			}
			for i, b := range boxes {
				if b.X < 0 || b.Y < 0 || b.X+b.W > translateResultW+1 || b.Y+b.H > clientH+1 {
					t.Fatalf("h=%d img=%v box %d outside %+v", clientH, img, i, b)
				}
			}
			if lay.CopyOriginal.overlaps(lay.Chinese) || lay.CopyOriginal.overlaps(lay.Image) || lay.Original.overlaps(lay.Chinese) {
				t.Fatalf("h=%d img=%v overlap %+v", clientH, img, lay)
			}
			if lay.Image.H > 0 && lay.Original.Y < lay.Image.Y+lay.Image.H {
				t.Fatalf("h=%d img=%v text covers image %+v", clientH, img, lay)
			}
		}
	}
}

func TestFitImage(t *testing.T) {
	w, h := fitImage(800, 400, 528, 200)
	if w > 528 || h > 200 || w <= 0 || h <= 0 {
		t.Fatalf("fit = %d %d", w, h)
	}
	if fw, fh := fitImage(0, 10, 100, 100); fw != 0 || fh != 0 {
		t.Fatalf("empty = %d %d", fw, fh)
	}
}

func TestBullHeadAsset(t *testing.T) {
	px, w, h, err := decodePremulBGRA(bullHeadPNG)
	if err != nil {
		t.Fatal(err)
	}
	if w < 64 || h < 64 || len(px) != int(w*h*4) {
		t.Fatalf("bull %dx%d len %d", w, h, len(px))
	}
	// Corners of the crystal bull are transparent. The head is not.
	if px[3] != 0 {
		t.Fatalf("corner alpha = %d", px[3])
	}
	mid := (int(h/2)*int(w) + int(w/2)) * 4
	if px[mid+3] == 0 {
		t.Fatal("center of the bull is empty")
	}
}

func TestDecodePremulBGRA(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Pix = []byte{255, 0, 0, 255}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	px, w, h, err := decodePremulBGRA(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if w != 1 || h != 1 || px[0] != 0 || px[1] != 0 || px[2] != 255 || px[3] != 255 {
		t.Fatalf("px=%v %dx%d", px, w, h)
	}
}
