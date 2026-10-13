package bridge

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
	"strings"
)

//go:embed bull_head.png
var bullHeadPNG []byte

const (
	floatWidgetW     int32 = 460
	floatWidgetH     int32 = 236
	collapsedBull    int32 = 72
	translateResultW int32 = 560
)

// pxBox is a client-area rectangle shared by the desktop widget and the result window.
type pxBox struct {
	X, Y, W, H int32
}

func (b pxBox) contains(x, y int32) bool {
	return x >= b.X && y >= b.Y && x < b.X+b.W && y < b.Y+b.H
}

func (b pxBox) overlaps(o pxBox) bool {
	if b.W <= 0 || b.H <= 0 || o.W <= 0 || o.H <= 0 {
		return false
	}
	return b.X < o.X+o.W && o.X < b.X+b.W && b.Y < o.Y+o.H && o.Y < b.Y+b.H
}

type floatWidgetLayout struct {
	Bull, Pin, Close, Shot, Translate pxBox
}

// layoutFloatWidget places the always-on-top desktop widget.
// The two cards sit side by side. Shot is Ctrl+Alt+A. Translate is Ctrl+Alt+T.
// Close is the 关闭 button. Closing collapses to a bull icon; it does not destroy the window.
func layoutFloatWidget(w, h int32) floatWidgetLayout {
	closeBox := pxBox{w - 16 - 72, 12, 72, 32}
	gap := int32(12)
	pad := int32(16)
	cardY := int32(56)
	cardH := h - cardY - 16
	if cardH < 100 {
		cardH = 100
	}
	cardW := (w - pad*2 - gap) / 2
	return floatWidgetLayout{
		Bull:      pxBox{16, 8, 40, 40},
		Pin:       pxBox{closeBox.X - 8 - 64, 12, 64, 32},
		Close:     closeBox,
		Shot:      pxBox{pad, cardY, cardW, cardH},
		Translate: pxBox{pad + cardW + gap, cardY, cardW, cardH},
	}
}

type widgetFrame struct {
	X, Y, W, H int32
	Collapsed  bool
}

// collapseWidgetFrame parks a small bull at the expanded window's bottom-right.
func collapseWidgetFrame(cur widgetFrame) widgetFrame {
	w, h := cur.W, cur.H
	if w < collapsedBull {
		w = collapsedBull
	}
	if h < collapsedBull {
		h = collapsedBull
	}
	return widgetFrame{
		X:         cur.X + w - collapsedBull,
		Y:         cur.Y + h - collapsedBull,
		W:         collapsedBull,
		H:         collapsedBull,
		Collapsed: true,
	}
}

// expandWidgetFrame restores the full widget at the last expanded position.
func expandWidgetFrame(homeX, homeY int32) widgetFrame {
	return widgetFrame{X: homeX, Y: homeY, W: floatWidgetW, H: floatWidgetH, Collapsed: false}
}

// clickExpandsCollapsed is a click on the bull icon. A drag of 4px or more moves it instead.
func clickExpandsCollapsed(dx, dy int32) bool {
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx < 4 && dy < 4
}

// stackResultAbove places the result window on top of the expanded widget, right edges aligned.
func stackResultAbove(widgetX, widgetY, widgetW, resultW, resultH, workLeft, workTop, workRight, workBottom int32, collapsed bool) (int32, int32) {
	var x, y int32
	if !collapsed && widgetW > 0 {
		x = widgetX + widgetW - resultW
		y = widgetY - resultH - 10
	} else {
		x = workRight - resultW - 20
		y = workTop + 24
	}
	if x+resultW > workRight-8 {
		x = workRight - resultW - 8
	}
	if y+resultH > workBottom-8 {
		y = workBottom - resultH - 8
	}
	if x < workLeft+8 {
		x = workLeft + 8
	}
	if y < workTop+8 {
		y = workTop + 8
	}
	return x, y
}

func widgetHotkeyLabel(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	return raw
}

// desktopWidgetText is the chrome of the floating widget. It has no translation history.
func desktopWidgetText(shot, translate string) string {
	shot = widgetHotkeyLabel(shot, "Ctrl+Alt+A")
	translate = widgetHotkeyLabel(translate, "Ctrl+Alt+T")
	return strings.Join([]string{
		"截图",
		"置顶",
		"关闭",
		"截图",
		shot,
		"截图翻译",
		translate,
	}, "\n")
}

type translateResultCopy struct {
	Title, ImageLabel           string
	OriginalLabel, ChineseLabel string
	CopyOriginal, CopyChinese   string
	Pin, Close                  string
	Original, Chinese, Note     string
	HasImage                    bool
}

func (v translateResultCopy) chromeText() string {
	return strings.Join([]string{
		v.Title,
		v.ImageLabel,
		v.OriginalLabel,
		v.ChineseLabel,
		v.CopyOriginal,
		v.CopyChinese,
		v.Pin,
		v.Close,
		v.Note,
	}, "\n")
}

// composeTranslateResult is the separate result window: captured image, 原文, 中文, and copy.
// It never adds a history list.
func composeTranslateResult(original, chinese, errText, provider string, hasImage bool) translateResultCopy {
	note := providerNote(provider)
	zh := strings.TrimSpace(chinese)
	if errText = strings.TrimSpace(errText); errText != "" {
		if zh == "" {
			zh = errText
		} else {
			note = errText
		}
	}
	imageLabel := ""
	if hasImage {
		imageLabel = "截到的画面"
	}
	return translateResultCopy{
		Title:         "截图翻译",
		ImageLabel:    imageLabel,
		OriginalLabel: "原文",
		ChineseLabel:  "中文",
		CopyOriginal:  "复制原文",
		CopyChinese:   "复制译文",
		Pin:           "置顶",
		Close:         "关闭",
		Original:      strings.TrimSpace(original),
		Chinese:       zh,
		Note:          note,
		HasImage:      hasImage,
	}
}

func providerNote(provider string) string {
	switch provider {
	case "deepseek":
		return "来源：DeepSeek"
	case "google":
		return "来源：谷歌"
	case "mymemory":
		return "来源：MyMemory"
	default:
		return ""
	}
}

func containsRecentHistory(text string) bool {
	return strings.Contains(text, "最近翻译") || strings.Contains(text, "最近的翻译")
}

func fitImage(imgW, imgH, maxW, maxH int32) (int32, int32) {
	if imgW <= 0 || imgH <= 0 || maxW <= 0 || maxH <= 0 {
		return 0, 0
	}
	dw, dh := imgW, imgH
	if dw > maxW {
		dh = int32(int64(dh) * int64(maxW) / int64(dw))
		dw = maxW
	}
	if dh > maxH {
		if dh <= 0 {
			return 0, 0
		}
		dw = int32(int64(dw) * int64(maxH) / int64(dh))
		dh = maxH
	}
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	return dw, dh
}

func clampTextH(h int32) int32 {
	if h < 40 {
		return 40
	}
	if h > 140 {
		return 140
	}
	return h
}

type translateResultLayout struct {
	Pin, Close                      pxBox
	ImageLabel, Image               pxBox
	OriginalLabel, Original         pxBox
	ChineseLabel, Chinese           pxBox
	CopyOriginal, CopyChinese, Note pxBox
}

func translateResultHeight(imgW, imgH, originalH, chineseH, maxH int32) int32 {
	h := int32(56)
	if imgW > 0 && imgH > 0 {
		_, dh := fitImage(imgW, imgH, translateResultW-32, 200)
		h += 24 + dh + 12
	}
	h += 24 + clampTextH(originalH) + 8 + 24 + clampTextH(chineseH) + 12 + 42 + 8 + 22 + 14
	if h < 300 {
		h = 300
	}
	if maxH > 0 && h > maxH {
		h = maxH
	}
	return h
}

// layoutTranslateResult keeps the picture, the two text blocks, and the copy buttons in one window.
func layoutTranslateResult(clientW, clientH, imgW, imgH, originalH, chineseH int32) translateResultLayout {
	var lay translateResultLayout
	if clientW < 240 {
		clientW = 240
	}
	if clientH < 300 {
		clientH = 300
	}
	lay.Close = pxBox{clientW - 16 - 72, 12, 72, 32}
	lay.Pin = pxBox{lay.Close.X - 8 - 64, 12, 64, 32}
	lay.Note = pxBox{16, clientH - 32, clientW - 32, 20}
	btnY := lay.Note.Y - 8 - 42
	gap := int32(10)
	bw := (clientW - 32 - gap) / 2
	lay.CopyOriginal = pxBox{16, btnY, bw, 42}
	lay.CopyChinese = pxBox{16 + bw + gap, btnY, bw, 42}
	y := int32(56)
	contentBottom := btnY - 8
	const textChrome int32 = 20 + 8 + 20
	if imgW > 0 && imgH > 0 {
		roomForImage := contentBottom - y - textChrome - 56 - 24 - 10
		if roomForImage > 48 {
			maxH := roomForImage
			if maxH > 200 {
				maxH = 200
			}
			dw, dh := fitImage(imgW, imgH, clientW-32, maxH)
			lay.ImageLabel = pxBox{16, y, clientW - 32, 22}
			y += 24
			lay.Image = pxBox{16, y, dw, dh}
			y += dh + 10
		}
	}
	lay.OriginalLabel = pxBox{16, y, clientW - 32, 20}
	y += 22
	textRoom := contentBottom - y - 8 - 20
	if textRoom < 48 {
		textRoom = 48
	}
	oh := clampTextH(originalH)
	zh := clampTextH(chineseH)
	if oh+zh > textRoom {
		oh = textRoom / 2
		zh = textRoom - oh
	}
	if oh < 1 {
		oh = 1
	}
	if zh < 1 {
		zh = 1
	}
	lay.Original = pxBox{16, y, clientW - 32, oh}
	y += oh + 8
	lay.ChineseLabel = pxBox{16, y, clientW - 32, 20}
	y += 22
	if y+zh > contentBottom && contentBottom > y {
		zh = contentBottom - y
	}
	if zh < 1 {
		zh = 1
	}
	lay.Chinese = pxBox{16, y, clientW - 32, zh}
	return lay
}

func decodePremulBGRA(data []byte) ([]byte, int32, int32, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, 0, 0, errNoShotText
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	out := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		r := uint32(dst.Pix[i*4+0])
		g := uint32(dst.Pix[i*4+1])
		bl := uint32(dst.Pix[i*4+2])
		a := uint32(dst.Pix[i*4+3])
		out[i*4+0] = byte(bl * a / 255)
		out[i*4+1] = byte(g * a / 255)
		out[i*4+2] = byte(r * a / 255)
		out[i*4+3] = byte(a)
	}
	return out, int32(w), int32(h), nil
}
