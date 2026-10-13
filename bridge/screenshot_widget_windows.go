//go:build windows

package bridge

import (
	"fmt"
	"log"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	swHide             = 0
	wsExNoActivate     = 0x08000000
	wmMouseActivate    = 0x0021
	maNoActivate       = 3
	kindWidget         = 5
	kindResult         = 6
	spiGetWorkArea     = 0x0030
	widgetShotID       = 1
	widgetTranslateID  = 2
	widgetPinID        = 3
	resultCopyOriginal = 21
	resultCopyChinese  = 22
	resultPinID        = 23
	resultCloseID      = 24
	accentAcrylic      = 4
	wcaAccentPolicy    = 19
	dwmwaCornerPref    = 33
	dwmRound           = 2
	cfUnicodeText      = 13
)

type accentPolicy struct {
	State int32
	Flags int32
	Color uint32
	Anim  int32
}

type compositionData struct {
	Attribute uint32
	Data      uintptr
	Size      uintptr
}

const (
	_ = uint64(16 - unsafe.Sizeof(accentPolicy{}))
	_ = uint64(unsafe.Sizeof(accentPolicy{}) - 16)
	_ = uint64(24 - unsafe.Sizeof(compositionData{}))
	_ = uint64(unsafe.Sizeof(compositionData{}) - 24)
)

type floatWidget struct {
	hwnd                 uintptr
	acrylic              bool
	drag                 bool
	collapsed            bool
	dragMoved            bool
	offX, offY           int32
	pressX, pressY       int32
	homeX, homeY         int32
	shotHotkey, trHotkey string
	bull                 uintptr
	bullW, bullH         int32
}

type resultWindow struct {
	hwnd              uintptr
	original, chinese string
	note, status      string
	img               uintptr
	imgW, imgH        int32
	acrylic           bool
	drag              bool
	offX, offY        int32
	layout            translateResultLayout
}

var (
	dwmapi                        = windows.NewLazySystemDLL("dwmapi.dll")
	setWindowCompositionAttribute = user32.NewProc("SetWindowCompositionAttribute")
	systemParametersInfo          = user32.NewProc("SystemParametersInfoW")
	createDIBSection              = gdi32.NewProc("CreateDIBSection")
	setWindowText                 = user32.NewProc("SetWindowTextW")
	dwmSetWindowAttribute         = dwmapi.NewProc("DwmSetWindowAttribute")

	liveWidget          *floatWidget
	liveResult          *resultWindow
	resultHiddenForShot bool
	busyHiddenForShot   bool
	glassDC             uintptr
	glassBmp            uintptr
)

func createFloatWidget() error {
	if liveWidget != nil {
		return nil
	}
	x, y := widgetOrigin()
	hwnd := createShotWindow(wsExTopmost|wsExTool|wsExNoActivate, wsPopup|wsVisible, x, y, floatWidgetW, floatWidgetH)
	if hwnd == 0 {
		return fmt.Errorf("创建悬浮窗失败：%v", windows.GetLastError())
	}
	if title, err := windows.UTF16PtrFromString("截图"); err == nil {
		setWindowText.Call(hwnd, uintptr(unsafe.Pointer(title)))
	}
	roundWindowCorners(hwnd)
	w := &floatWidget{hwnd: hwnd, shotHotkey: "Ctrl+Alt+A", trHotkey: "Ctrl+Alt+T", acrylic: enableAcrylic(hwnd), homeX: x, homeY: y}
	if px, bw, bh, err := decodePremulBGRA(bullHeadPNG); err == nil {
		w.bullW, w.bullH = bw, bh
		w.bull = dibFromPremul(px, bw, bh)
	} else {
		log.Printf("screenshot bull: %v", err)
	}
	liveWidget = w
	windowKind[hwnd] = kindWidget
	showWindow.Call(hwnd, swShow)
	raiseTop(hwnd)
	return nil
}

func hideFloatWidget() {
	if liveWidget != nil && liveWidget.hwnd != 0 {
		showWindow.Call(liveWidget.hwnd, swHide)
	}
}

func hideResultForCapture() {
	if liveResult != nil && liveResult.hwnd != 0 {
		showWindow.Call(liveResult.hwnd, swHide)
		resultHiddenForShot = true
	}
}

func hideBusyForCapture() {
	if busyHwnd != 0 {
		showWindow.Call(busyHwnd, swHide)
		busyHiddenForShot = true
	}
}

func showFloatWidget() {
	if liveWidget == nil || liveWidget.hwnd == 0 {
		return
	}
	showWindow.Call(liveWidget.hwnd, swShow)
	raiseTop(liveWidget.hwnd)
	if resultHiddenForShot && liveResult != nil && liveResult.hwnd != 0 {
		showWindow.Call(liveResult.hwnd, swShow)
	}
	resultHiddenForShot = false
	if busyHiddenForShot && busyHwnd != 0 {
		showWindow.Call(busyHwnd, swShow)
	}
	busyHiddenForShot = false
	raiseShotPeers()
}

func raiseTop(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	setWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShow)
}

func raiseShotPeers() {
	for hwnd := range pins {
		raiseTop(hwnd)
	}
	if liveResult != nil {
		raiseTop(liveResult.hwnd)
	}
	if livePopup != nil {
		raiseTop(livePopup.hwnd)
	}
	if busyHwnd != 0 {
		raiseTop(busyHwnd)
	}
	if liveOverlay != nil {
		raiseTop(liveOverlay.hwnd)
	}
}

func rememberWidgetHotkeys(shot, translate string) {
	if liveWidget == nil {
		return
	}
	if label := strings.TrimSpace(shot); label != "" {
		liveWidget.shotHotkey = label
	}
	if label := strings.TrimSpace(translate); label != "" {
		liveWidget.trHotkey = label
	}
	invalidateRect.Call(liveWidget.hwnd, 0, 0)
}

func widgetOrigin() (int32, int32) {
	work := primaryWorkArea()
	x := work.Right - floatWidgetW - 20
	y := work.Bottom - floatWidgetH - 20
	if x < work.Left+8 {
		x = work.Left + 8
	}
	if y < work.Top+8 {
		y = work.Top + 8
	}
	return x, y
}

func resultMaxHeight() int32 {
	work := primaryWorkArea()
	h := work.Bottom - work.Top - floatWidgetH - 48
	if h < 420 {
		h = 420
	}
	if h > 760 {
		h = 760
	}
	return h
}

func resultOrigin(w, h int32) (int32, int32) {
	work := primaryWorkArea()
	collapsed := true
	var wx, wy, ww int32
	if liveWidget != nil && liveWidget.hwnd != 0 && !liveWidget.collapsed {
		var rc rect
		r1, _, _ := getWindowRect.Call(liveWidget.hwnd, uintptr(unsafe.Pointer(&rc)))
		if r1 != 0 {
			collapsed = false
			wx, wy = rc.Left, rc.Top
			ww = rc.Right - rc.Left
		}
	}
	return stackResultAbove(wx, wy, ww, w, h, work.Left, work.Top, work.Right, work.Bottom, collapsed)
}

func placeWidget(x, y, w, h int32) {
	if liveWidget == nil || liveWidget.hwnd == 0 {
		return
	}
	setWindowPos.Call(liveWidget.hwnd, hwndTopmost, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoActivate|swpShow)
	invalidateRect.Call(liveWidget.hwnd, 0, 1)
}

func collapseFloatWidget() {
	w := liveWidget
	if w == nil || w.hwnd == 0 || w.collapsed {
		return
	}
	var rc rect
	getWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&rc)))
	ww, hh := rc.Right-rc.Left, rc.Bottom-rc.Top
	if ww < collapsedBull {
		ww = floatWidgetW
	}
	if hh < collapsedBull {
		hh = floatWidgetH
	}
	w.homeX, w.homeY = rc.Left, rc.Top
	frame := collapseWidgetFrame(widgetFrame{X: rc.Left, Y: rc.Top, W: ww, H: hh})
	w.collapsed = true
	w.drag = false
	placeWidget(frame.X, frame.Y, frame.W, frame.H)
}

func expandFloatWidget() {
	w := liveWidget
	if w == nil || w.hwnd == 0 || !w.collapsed {
		return
	}
	w.collapsed = false
	w.drag = false
	placeWidget(w.homeX, w.homeY, floatWidgetW, floatWidgetH)
}

func primaryWorkArea() rect {
	var rc rect
	if call(systemParametersInfo, spiGetWorkArea, 0, uintptr(unsafe.Pointer(&rc)), 0) != 0 && rc.Right > rc.Left && rc.Bottom > rc.Top {
		return rc
	}
	left := int32(call(getSystemMetrics, smXVirtualScreen))
	top := int32(call(getSystemMetrics, smYVirtualScreen))
	return rect{
		Left:   left,
		Top:    top,
		Right:  left + int32(call(getSystemMetrics, smCXVirtualScreen)),
		Bottom: top + int32(call(getSystemMetrics, smCYVirtualScreen)),
	}
}

func enableAcrylic(hwnd uintptr) bool {
	if setWindowCompositionAttribute.Find() != nil {
		return false
	}
	policy := accentPolicy{State: accentAcrylic, Flags: 2, Color: 0xD0241208}
	data := compositionData{Attribute: wcaAccentPolicy, Data: uintptr(unsafe.Pointer(&policy)), Size: unsafe.Sizeof(policy)}
	return call(setWindowCompositionAttribute, hwnd, uintptr(unsafe.Pointer(&data))) != 0
}

func roundWindowCorners(hwnd uintptr) {
	if dwmSetWindowAttribute.Find() != nil {
		return
	}
	pref := int32(dwmRound)
	dwmSetWindowAttribute.Call(hwnd, dwmwaCornerPref, uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
}

func dibFromPremul(px []byte, w, h int32) uintptr {
	if w <= 0 || h <= 0 || len(px) < int(w*h*4) {
		return 0
	}
	hdr := bitmapInfoHeader{
		BiSize: 40, BiWidth: w, BiHeight: -h, BiPlanes: 1, BiBitCount: 32,
		BiCompression: biRGB, BiSizeImage: uint32(w * h * 4),
	}
	var bits unsafe.Pointer
	bmp := call(createDIBSection, 0, uintptr(unsafe.Pointer(&hdr)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return 0
	}
	dst := unsafe.Slice((*byte)(bits), int(w*h*4))
	copy(dst, px[:int(w*h*4)])
	return bmp
}

func drawPremul(hdc, bmp uintptr, sw, sh, x, y, dw, dh int32) {
	if bmp == 0 || sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 {
		return
	}
	mem := call(createCompatibleDC, hdc)
	if mem == 0 {
		return
	}
	old := call(selectObject, mem, bmp)
	const srcAlpha = uintptr(255)<<16 | uintptr(1)<<24
	if call(alphaBlend, hdc, uintptr(x), uintptr(y), uintptr(dw), uintptr(dh), mem, 0, 0, uintptr(sw), uintptr(sh), srcAlpha) == 0 {
		setStretchBltMode.Call(hdc, colorOnColor)
		stretchBlt.Call(hdc, uintptr(x), uintptr(y), uintptr(dw), uintptr(dh), mem, 0, 0, uintptr(sw), uintptr(sh), srcCopy)
	}
	selectObject.Call(mem, old)
	deleteDC.Call(mem)
}

func ensureGlass(hdc uintptr) {
	if glassDC != 0 {
		return
	}
	glassDC = call(createCompatibleDC, hdc)
	glassBmp = call(createCompatibleBitmap, hdc, 1, 1)
	if glassDC == 0 || glassBmp == 0 {
		return
	}
	selectObject.Call(glassDC, glassBmp)
	fillBox(glassDC, 0, 0, 1, 1, colorRef(8, 18, 36))
}

func glassFill(hdc uintptr, x, y, w, h int32, alpha byte) {
	ensureGlass(hdc)
	if glassDC == 0 || w <= 0 || h <= 0 {
		return
	}
	alphaBlend.Call(hdc, uintptr(x), uintptr(y), uintptr(w), uintptr(h), glassDC, 0, 0, 1, 1, uintptr(alpha)<<16)
}

func paintGlassChrome(hdc uintptr, w, h int32, acrylic bool) {
	if acrylic {
		glassFill(hdc, 0, 0, w, h, 96)
	} else {
		fillBox(hdc, 0, 0, w, h, colorRef(8, 18, 36))
	}
	strokeRect(hdc, 1, 1, w-2, h-2, colorRef(232, 213, 163), 1)
}

func paintGlassButton(hdc uintptr, b pxBox, border uintptr, acrylic bool) {
	if acrylic {
		glassFill(hdc, b.X, b.Y, b.W, b.H, 150)
	} else {
		fillBox(hdc, b.X, b.Y, b.W, b.H, colorRef(16, 36, 60))
	}
	strokeRect(hdc, b.X, b.Y, b.W, b.H, border, 1)
}

func paintCollapsedWidget(hdc uintptr, w *floatWidget) {
	paintGlassChrome(hdc, collapsedBull, collapsedBull, w.acrylic)
	if w.bull != 0 {
		drawPremul(hdc, w.bull, w.bullW, w.bullH, 8, 8, collapsedBull-16, collapsedBull-16)
	}
}

func paintFloatWidget(hdc uintptr) {
	w := liveWidget
	if w == nil {
		return
	}
	if w.collapsed {
		paintCollapsedWidget(hdc, w)
		return
	}
	paintGlassChrome(hdc, floatWidgetW, floatWidgetH, w.acrylic)
	lay := layoutFloatWidget(floatWidgetW, floatWidgetH)
	if w.bull != 0 {
		drawPremul(hdc, w.bull, w.bullW, w.bullH, lay.Bull.X, lay.Bull.Y, lay.Bull.W, lay.Bull.H)
	}
	titleW := lay.Pin.X - 64
	if titleW < 80 {
		titleW = 80
	}
	drawTextBox(hdc, "截图", 64, 12, titleW, 32, dtSingleLine|dtVCenter|dtNoPrefix, colorRef(232, 213, 163))
	paintGlassButton(hdc, lay.Pin, colorRef(159, 214, 255), w.acrylic)
	drawTextBox(hdc, "置顶", lay.Pin.X, lay.Pin.Y, lay.Pin.W, lay.Pin.H, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(159, 214, 255))
	paintGlassButton(hdc, lay.Close, colorRef(232, 213, 163), w.acrylic)
	drawTextBox(hdc, "关闭", lay.Close.X, lay.Close.Y, lay.Close.W, lay.Close.H, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(246, 241, 230))
	paintWidgetButton(hdc, lay.Shot, "截图", widgetHotkeyLabel(w.shotHotkey, "Ctrl+Alt+A"), colorRef(159, 214, 255), w)
	paintWidgetButton(hdc, lay.Translate, "截图翻译", widgetHotkeyLabel(w.trHotkey, "Ctrl+Alt+T"), colorRef(232, 213, 163), w)
}

func paintCopyButton(hdc uintptr, b pxBox, label string, border uintptr, acrylic bool) {
	paintGlassButton(hdc, b, border, acrylic)
	textX := b.X
	textW := b.W
	if liveWidget != nil && liveWidget.bull != 0 {
		drawPremul(hdc, liveWidget.bull, liveWidget.bullW, liveWidget.bullH, b.X+10, b.Y+(b.H-26)/2, 26, 26)
		textX = b.X + 40
		textW = b.W - 50
	}
	drawTextBox(hdc, label, textX, b.Y, textW, b.H, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(246, 241, 230))
}

func paintWidgetButton(hdc uintptr, b pxBox, title, hotkey string, border uintptr, w *floatWidget) {
	paintGlassButton(hdc, b, border, w.acrylic)
	bull := int32(48)
	if w.bull != 0 {
		drawPremul(hdc, w.bull, w.bullW, w.bullH, b.X+(b.W-bull)/2, b.Y+16, bull, bull)
	}
	titleY := b.Y + 16 + bull + 8
	drawTextBox(hdc, title, b.X+8, titleY, b.W-16, 32, dtCenter|dtSingleLine|dtNoPrefix, colorRef(246, 241, 230))
	drawTextBox(hdc, hotkey, b.X+8, titleY+32, b.W-16, 24, dtCenter|dtSingleLine|dtNoPrefix, colorRef(159, 214, 255))
}

func beginWidgetDrag(hwnd uintptr) {
	w := liveWidget
	if w == nil {
		return
	}
	var pt point
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	var rc rect
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	w.drag = true
	w.dragMoved = false
	w.pressX, w.pressY = pt.X, pt.Y
	w.offX = pt.X - rc.Left
	w.offY = pt.Y - rc.Top
	setCapture.Call(hwnd)
}

func onWidgetLeftDown(hwnd uintptr) {
	if liveWidget == nil {
		return
	}
	if liveWidget.collapsed {
		beginWidgetDrag(hwnd)
		return
	}
	x, y := clientCursor(hwnd)
	lay := layoutFloatWidget(floatWidgetW, floatWidgetH)
	switch {
	case lay.Shot.contains(x, y):
		liveWidget.drag = false
		beginCapture(modeShot)
	case lay.Translate.contains(x, y):
		liveWidget.drag = false
		beginCapture(modeTranslate)
	case lay.Close.contains(x, y):
		liveWidget.drag = false
		collapseFloatWidget()
	case lay.Pin.contains(x, y):
		showFloatWidget()
	default:
		beginWidgetDrag(hwnd)
	}
}

func onWidgetMouseMove(hwnd uintptr) {
	w := liveWidget
	if w == nil || !w.drag || w.hwnd != hwnd {
		return
	}
	var pt point
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if !clickExpandsCollapsed(pt.X-w.pressX, pt.Y-w.pressY) {
		w.dragMoved = true
	}
	x, y := pt.X-w.offX, pt.Y-w.offY
	var rc rect
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	ww, hh := rc.Right-rc.Left, rc.Bottom-rc.Top
	x, y = magnet(hwnd, x, y, ww, hh)
	setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
}

func onWidgetLeftUp(hwnd uintptr) {
	w := liveWidget
	if w == nil || w.hwnd != hwnd || !w.drag {
		return
	}
	moved := w.dragMoved
	collapsed := w.collapsed
	w.drag = false
	w.dragMoved = false
	releaseCapture.Call()
	if collapsed {
		if !moved {
			expandFloatWidget()
		}
		return
	}
	var rc rect
	r1, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	if r1 != 0 {
		w.homeX, w.homeY = rc.Left, rc.Top
	}
}

func releaseResult(r *resultWindow) {
	if r == nil {
		return
	}
	if r.img != 0 {
		deleteObject.Call(r.img)
		r.img = 0
	}
}

func showTranslateResult(item shotResult) {
	resultHiddenForShot = false
	if liveResult != nil {
		destroyWindow.Call(liveResult.hwnd)
	}
	var imgW, imgH int32
	var bmp uintptr
	if len(item.png) > 0 {
		if px, w, h, err := decodePremulBGRA(item.png); err == nil {
			bmp = dibFromPremul(px, w, h)
			if bmp != 0 {
				imgW, imgH = w, h
			}
		}
	}
	view := composeTranslateResult(item.original, item.chinese, item.errText, item.provider, bmp != 0)
	width := translateResultW
	oh := textBlockHeight(view.Original, width-32)
	zh := textBlockHeight(view.Chinese, width-32)
	height := translateResultHeight(imgW, imgH, oh, zh, resultMaxHeight())
	x, y := resultOrigin(width, height)
	hwnd := createShotWindow(wsExTopmost|wsExTool, wsPopup|wsVisible, x, y, width, height)
	if hwnd == 0 {
		if bmp != 0 {
			deleteObject.Call(bmp)
		}
		showPopup(item.x, item.y, item.original, item.chinese, item.errText, item.provider)
		return
	}
	if title, err := windows.UTF16PtrFromString("截图翻译"); err == nil {
		setWindowText.Call(hwnd, uintptr(unsafe.Pointer(title)))
	}
	roundWindowCorners(hwnd)
	liveResult = &resultWindow{
		hwnd:     hwnd,
		original: view.Original,
		chinese:  view.Chinese,
		note:     view.Note,
		img:      bmp,
		imgW:     imgW,
		imgH:     imgH,
		acrylic:  enableAcrylic(hwnd),
		layout:   layoutTranslateResult(width, height, imgW, imgH, oh, zh),
	}
	windowKind[hwnd] = kindResult
	showWindow.Call(hwnd, swShow)
	raiseTop(hwnd)
	setForegroundWindow.Call(hwnd)
	setFocus.Call(hwnd)
}

func textBlockHeight(text string, width int32) int32 {
	if strings.TrimSpace(text) == "" {
		return 40
	}
	_, h := measureBody(text, width)
	return h
}

func paintTranslateResult(hdc uintptr) {
	r := liveResult
	if r == nil {
		return
	}
	var cr rect
	getClientRect.Call(r.hwnd, uintptr(unsafe.Pointer(&cr)))
	paintGlassChrome(hdc, cr.Right, cr.Bottom, r.acrylic)
	lay := r.layout
	if liveWidget != nil && liveWidget.bull != 0 {
		drawPremul(hdc, liveWidget.bull, liveWidget.bullW, liveWidget.bullH, 14, 8, 32, 32)
	}
	drawTextBox(hdc, "截图翻译", 52, 10, lay.Pin.X-60, 30, dtSingleLine|dtVCenter|dtNoPrefix, colorRef(232, 213, 163))
	paintGlassButton(hdc, lay.Pin, colorRef(159, 214, 255), r.acrylic)
	drawTextBox(hdc, "置顶", lay.Pin.X, lay.Pin.Y, lay.Pin.W, lay.Pin.H, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(159, 214, 255))
	paintGlassButton(hdc, lay.Close, colorRef(232, 213, 163), r.acrylic)
	drawTextBox(hdc, "关闭", lay.Close.X, lay.Close.Y, lay.Close.W, lay.Close.H, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(246, 241, 230))
	if r.img != 0 && lay.Image.W > 0 {
		drawTextBox(hdc, "截到的画面", lay.ImageLabel.X, lay.ImageLabel.Y, lay.ImageLabel.W, lay.ImageLabel.H, dtSingleLine|dtVCenter|dtNoPrefix, colorRef(232, 213, 163))
		drawPremul(hdc, r.img, r.imgW, r.imgH, lay.Image.X, lay.Image.Y, lay.Image.W, lay.Image.H)
		strokeRect(hdc, lay.Image.X, lay.Image.Y, lay.Image.W, lay.Image.H, colorRef(232, 213, 163), 1)
	}
	drawTextBox(hdc, "原文", lay.OriginalLabel.X, lay.OriginalLabel.Y, lay.OriginalLabel.W, lay.OriginalLabel.H, dtSingleLine|dtVCenter|dtNoPrefix, colorRef(232, 213, 163))
	drawTextBox(hdc, r.original, lay.Original.X, lay.Original.Y, lay.Original.W, lay.Original.H, dtWordBreak|dtNoPrefix, colorRef(246, 241, 230))
	drawTextBox(hdc, "中文", lay.ChineseLabel.X, lay.ChineseLabel.Y, lay.ChineseLabel.W, lay.ChineseLabel.H, dtSingleLine|dtVCenter|dtNoPrefix, colorRef(232, 213, 163))
	drawTextBox(hdc, r.chinese, lay.Chinese.X, lay.Chinese.Y, lay.Chinese.W, lay.Chinese.H, dtWordBreak|dtNoPrefix, colorRef(246, 241, 230))
	paintCopyButton(hdc, lay.CopyOriginal, "复制原文", colorRef(159, 214, 255), r.acrylic)
	paintCopyButton(hdc, lay.CopyChinese, "复制译文", colorRef(232, 213, 163), r.acrylic)
	note := r.note
	if r.status != "" {
		note = r.status
	}
	if note != "" {
		drawTextBox(hdc, note, lay.Note.X, lay.Note.Y, lay.Note.W, lay.Note.H, dtSingleLine|dtNoPrefix, colorRef(159, 214, 255))
	}
}

func onResultLeftDown(hwnd uintptr) {
	r := liveResult
	if r == nil || r.hwnd != hwnd {
		return
	}
	x, y := clientCursor(hwnd)
	lay := r.layout
	switch {
	case lay.Close.contains(x, y):
		destroyWindow.Call(hwnd)
	case lay.Pin.contains(x, y):
		raiseTop(hwnd)
	case lay.CopyOriginal.contains(x, y):
		copyResultText(r, r.original, "原文已复制")
	case lay.CopyChinese.contains(x, y):
		copyResultText(r, r.chinese, "译文已复制")
	case y < 48:
		var pt point
		getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		var rc rect
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		r.drag = true
		r.offX = pt.X - rc.Left
		r.offY = pt.Y - rc.Top
		setCapture.Call(hwnd)
	}
}

func copyResultText(r *resultWindow, text, done string) {
	if err := copyUnicodeText(r.hwnd, text); err != nil {
		r.status = err.Error()
	} else {
		r.status = done
	}
	invalidateRect.Call(r.hwnd, 0, 0)
}

func onResultMouseMove(hwnd uintptr) {
	r := liveResult
	if r == nil || !r.drag || r.hwnd != hwnd {
		return
	}
	var pt point
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	x, y := pt.X-r.offX, pt.Y-r.offY
	var rc rect
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	x, y = magnet(hwnd, x, y, rc.Right-rc.Left, rc.Bottom-rc.Top)
	setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
}

func onResultLeftUp(hwnd uintptr) {
	r := liveResult
	if r == nil || r.hwnd != hwnd || !r.drag {
		return
	}
	r.drag = false
	releaseCapture.Call()
}

func copyUnicodeText(owner uintptr, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("没有可以复制的文字")
	}
	utf := windows.StringToUTF16(text)
	hglob := call(globalAlloc, gmemMoveable, uintptr(len(utf)*2))
	if hglob == 0 {
		return fmt.Errorf("复制失败")
	}
	ptr := call(globalLock, hglob)
	if ptr == 0 {
		globalFree.Call(hglob)
		return fmt.Errorf("复制失败")
	}
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf))
	copy(dst, utf)
	globalUnlock.Call(hglob)
	if call(openClipboard, owner) == 0 {
		globalFree.Call(hglob)
		return fmt.Errorf("复制失败：剪贴板被占用")
	}
	emptyClipboard.Call()
	if call(setClipboardData, cfUnicodeText, hglob) == 0 {
		closeClipboard.Call()
		globalFree.Call(hglob)
		return fmt.Errorf("复制失败")
	}
	closeClipboard.Call()
	return nil
}

func showShotResult(item shotResult) {
	if len(item.png) > 0 || strings.TrimSpace(item.original) != "" || strings.TrimSpace(item.chinese) != "" {
		showTranslateResult(item)
		return
	}
	showPopup(item.x, item.y, item.original, item.chinese, item.errText, item.provider)
}
