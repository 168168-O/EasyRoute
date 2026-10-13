//go:build windows

package bridge

import (
	"errors"
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmDestroy       = 0x0002
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmKeyDown       = 0x0100
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmMouseWheel    = 0x020A
	wmHotkey        = 0x0312
	wmShotCmd       = 0x8001
	wmShotResult    = 0x8002

	wsPopup     = 0x80000000
	wsVisible   = 0x10000000
	wsExTopmost = 0x00000008
	wsExTool    = 0x00000080
	wsExLayered = 0x00080000

	swShow         = 5
	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
	swpShow        = 0x0040
	lwaAlpha       = 0x00000002
	mkControl      = 0x0008
	nullBrush      = 5
	colorOnColor   = 3
	csDblClks      = 0x0008
	csHRedraw      = 0x0002
	csVRedraw      = 0x0001
	idcArrow       = 32512
	vkEscape       = 0x1B
	tpmReturnCmd   = 0x0100
	tpmNoNotify    = 0x0080
	monitorNearest = 2

	dtCenter     = 0x00000001
	dtVCenter    = 0x00000004
	dtWordBreak  = 0x00000010
	dtSingleLine = 0x00000020
	dtNoPrefix   = 0x00000800
	dtCalcRect   = 0x00000400

	modeShot       = 0
	modeTranslate  = 1
	phaseSelecting = 0
	phaseToolbar   = 1
	kindOverlay    = 1
	kindPin        = 2
	kindPopup      = 3
	kindBusy       = 4
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type winMSG struct {
	Hwnd    uintptr
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
	Private uint32
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type point struct {
	X, Y int32
}

type paintStruct struct {
	Hdc       uintptr
	Erase     int32
	RcLeft    int32
	RcTop     int32
	RcRight   int32
	RcBottom  int32
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor [4]int32
	RcWork    [4]int32
	DwFlags   uint32
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type openFileName struct {
	StructSize      uint32
	Owner           uintptr
	Instance        uintptr
	Filter          *uint16
	CustomFilter    *uint16
	MaxCustomFilter uint32
	FilterIndex     uint32
	File            *uint16
	MaxFile         uint32
	FileTitle       *uint16
	MaxFileTitle    uint32
	InitialDir      *uint16
	Title           *uint16
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefExt          *uint16
	CustData        uintptr
	FnHook          uintptr
	TemplateName    *uint16
	PvReserved      uintptr
	DwReserved      uint32
	FlagsEx         uint32
}

const (
	_ = uint64(80 - unsafe.Sizeof(wndClassEx{}))
	_ = uint64(unsafe.Sizeof(wndClassEx{}) - 80)
	_ = uint64(48 - unsafe.Sizeof(winMSG{}))
	_ = uint64(unsafe.Sizeof(winMSG{}) - 48)
	_ = uint64(40 - unsafe.Sizeof(monitorInfo{}))
	_ = uint64(unsafe.Sizeof(monitorInfo{}) - 40)
	_ = uint64(40 - unsafe.Sizeof(bitmapInfoHeader{}))
	_ = uint64(unsafe.Sizeof(bitmapInfoHeader{}) - 40)
	_ = uint64(152 - unsafe.Sizeof(openFileName{}))
	_ = uint64(unsafe.Sizeof(openFileName{}) - 152)
	_ = uint64(72 - unsafe.Sizeof(paintStruct{}))
	_ = uint64(unsafe.Sizeof(paintStruct{}) - 72)
)

type shotButton struct {
	label string
	x, y  int32
	w, h  int32
	id    int
}

type overlay struct {
	hwnd           uintptr
	hdcMem         uintptr
	bmp            uintptr
	oldBmp         uintptr
	vx, vy, vw, vh int32
	px             []byte
	mode           int
	phase          int
	dragging       bool
	x0, y0, x1, y1 int32
	buttons        []shotButton
}

type pin struct {
	hwnd       uintptr
	bmp        uintptr
	px         []byte
	imgW, imgH int32
	zoom       float64
	alpha      byte
	drag       bool
	offX, offY int32
}

type textPopup struct {
	hwnd     uintptr
	title    string
	original string
	chinese  string
	note     string
}

type shotCommand struct {
	shot, translate, key, proxy string
	reply                       chan FlagResult
}

type shotResult struct {
	id                          int
	original, chinese, provider string
	errText                     string
	x, y                        int32
	png                         []byte
}

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	msimg32  = windows.NewLazySystemDLL("msimg32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	comdlg32 = windows.NewLazySystemDLL("comdlg32.dll")

	registerClassEx            = user32.NewProc("RegisterClassExW")
	createWindowEx             = user32.NewProc("CreateWindowExW")
	defWindowProc              = user32.NewProc("DefWindowProcW")
	getMessage                 = user32.NewProc("GetMessageW")
	translateMessage           = user32.NewProc("TranslateMessage")
	dispatchMessage            = user32.NewProc("DispatchMessageW")
	postMessage                = user32.NewProc("PostMessageW")
	registerHotKey             = user32.NewProc("RegisterHotKey")
	unregisterHotKey           = user32.NewProc("UnregisterHotKey")
	destroyWindow              = user32.NewProc("DestroyWindow")
	showWindow                 = user32.NewProc("ShowWindow")
	setWindowPos               = user32.NewProc("SetWindowPos")
	getWindowRect              = user32.NewProc("GetWindowRect")
	getClientRect              = user32.NewProc("GetClientRect")
	beginPaint                 = user32.NewProc("BeginPaint")
	endPaint                   = user32.NewProc("EndPaint")
	fillRect                   = user32.NewProc("FillRect")
	invalidateRect             = user32.NewProc("InvalidateRect")
	setCapture                 = user32.NewProc("SetCapture")
	releaseCapture             = user32.NewProc("ReleaseCapture")
	getCursorPos               = user32.NewProc("GetCursorPos")
	setForegroundWindow        = user32.NewProc("SetForegroundWindow")
	setFocus                   = user32.NewProc("SetFocus")
	loadCursor                 = user32.NewProc("LoadCursorW")
	getSystemMetrics           = user32.NewProc("GetSystemMetrics")
	getDC                      = user32.NewProc("GetDC")
	releaseDC                  = user32.NewProc("ReleaseDC")
	monitorFromWindow          = user32.NewProc("MonitorFromWindow")
	getMonitorInfo             = user32.NewProc("GetMonitorInfoW")
	setLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	createPopupMenu            = user32.NewProc("CreatePopupMenu")
	appendMenu                 = user32.NewProc("AppendMenuW")
	trackPopupMenu             = user32.NewProc("TrackPopupMenu")
	destroyMenu                = user32.NewProc("DestroyMenu")
	openClipboard              = user32.NewProc("OpenClipboard")
	emptyClipboard             = user32.NewProc("EmptyClipboard")
	setClipboardData           = user32.NewProc("SetClipboardData")
	closeClipboard             = user32.NewProc("CloseClipboard")
	drawTextW                  = user32.NewProc("DrawTextW")

	createCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	selectObject           = gdi32.NewProc("SelectObject")
	bitBlt                 = gdi32.NewProc("BitBlt")
	stretchBlt             = gdi32.NewProc("StretchBlt")
	setStretchBltMode      = gdi32.NewProc("SetStretchBltMode")
	deleteDC               = gdi32.NewProc("DeleteDC")
	deleteObject           = gdi32.NewProc("DeleteObject")
	getDIBits              = gdi32.NewProc("GetDIBits")
	setDIBits              = gdi32.NewProc("SetDIBits")
	createPen              = gdi32.NewProc("CreatePen")
	createSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	rectangle              = gdi32.NewProc("Rectangle")
	patBlt                 = gdi32.NewProc("PatBlt")
	createFont             = gdi32.NewProc("CreateFontW")
	setBkMode              = gdi32.NewProc("SetBkMode")
	setTextColor           = gdi32.NewProc("SetTextColor")
	getStockObject         = gdi32.NewProc("GetStockObject")

	alphaBlend = msimg32.NewProc("AlphaBlend")

	getModuleHandle = kernel32.NewProc("GetModuleHandleW")
	globalAlloc     = kernel32.NewProc("GlobalAlloc")
	globalLock      = kernel32.NewProc("GlobalLock")
	globalUnlock    = kernel32.NewProc("GlobalUnlock")
	globalFree      = kernel32.NewProc("GlobalFree")

	getSaveFileName      = comdlg32.NewProc("GetSaveFileNameW")
	commDlgExtendedError = comdlg32.NewProc("CommDlgExtendedError")

	shotClassName = windows.StringToUTF16("DahuoShotWindow")
	shotWndProcCB uintptr
	shotInstance  uintptr
	hwndMessage   = ^uintptr(2)
	hwndTopmost   = ^uintptr(0)
	msgHwnd       uintptr
	shotStart     sync.Once
	readyOnce     sync.Once
	readyCh       = make(chan struct{})
	loopErr       error
	fontHandle    uintptr
	dimDC         uintptr
	dimBmp        uintptr
	liveOverlay   *overlay
	livePopup     *textPopup
	busyHwnd      uintptr
	currentJob    int
	pins          = map[uintptr]*pin{}
	windowKind    = map[uintptr]int{}
	cmdMu         sync.Mutex
	cmds          []shotCommand
	resultMu      sync.Mutex
	resultQ       []shotResult
	secretMu      sync.Mutex
	secretKey     string
	secretProxy   string
)

func call(p *windows.LazyProc, args ...uintptr) uintptr {
	r, _, _ := p.Call(args...)
	return r
}

func colorRef(r, g, b byte) uintptr {
	return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}

func signalReady() {
	readyOnce.Do(func() { close(readyCh) })
}

func ensureShotLoop() error {
	shotStart.Do(func() { go shotLoopMain() })
	select {
	case <-readyCh:
		return loopErr
	case <-time.After(4 * time.Second):
		return errors.New("截图服务没有响应")
	}
}

func shotLoopMain() {
	runtime.LockOSThread()
	defer signalReady()
	if err := startShotWindow(); err != nil {
		loopErr = err
		log.Printf("screenshot: %v", err)
		return
	}
	signalReady()
	pump()
}

func startShotWindow() error {
	shotWndProcCB = windows.NewCallback(shotWndProc)
	shotInstance = call(getModuleHandle, 0)
	wc := wndClassEx{
		Style:     csDblClks | csHRedraw | csVRedraw,
		WndProc:   shotWndProcCB,
		Instance:  shotInstance,
		Cursor:    call(loadCursor, 0, idcArrow),
		ClassName: &shotClassName[0],
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if call(registerClassEx, uintptr(unsafe.Pointer(&wc))) == 0 {
		return fmt.Errorf("注册窗口类失败：%v", windows.GetLastError())
	}
	msgHwnd = call(createWindowEx, 0, uintptr(unsafe.Pointer(&shotClassName[0])), 0, 0, 0, 0, 0, 0, hwndMessage, 0, shotInstance, 0)
	if msgHwnd == 0 {
		return fmt.Errorf("创建消息窗口失败：%v", windows.GetLastError())
	}
	if err := createFloatWidget(); err != nil {
		log.Printf("screenshot widget: %v", err)
	}
	return nil
}

func pump() {
	for {
		var msg winMSG
		r := call(getMessage, uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(r) {
		case -1:
			continue
		case 0:
			return
		}
		switch msg.Message {
		case wmShotCmd:
			drainCmds()
			continue
		case wmShotResult:
			drainResults()
			continue
		case wmHotkey:
			onHotkey(msg.WParam)
			continue
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func applyScreenshotHotkeys(shot, translate, deepseekKey, coreProxy string) FlagResult {
	if err := ensureShotLoop(); err != nil {
		return FlagResult{false, err.Error()}
	}
	reply := make(chan FlagResult, 1)
	cmdMu.Lock()
	cmds = append(cmds, shotCommand{shot: shot, translate: translate, key: deepseekKey, proxy: coreProxy, reply: reply})
	cmdMu.Unlock()
	postMessage.Call(msgHwnd, wmShotCmd, 0, 0)
	select {
	case res := <-reply:
		return res
	case <-time.After(4 * time.Second):
		return FlagResult{false, "截图服务没有响应"}
	}
}

func drainCmds() {
	cmdMu.Lock()
	batch := cmds
	cmds = nil
	cmdMu.Unlock()
	for _, cmd := range batch {
		setSecrets(cmd.key, cmd.proxy)
		cmd.reply <- registerShotHotkeys(cmd.shot, cmd.translate)
	}
}

func setSecrets(key, proxy string) {
	secretMu.Lock()
	secretKey, secretProxy = strings.TrimSpace(key), strings.TrimSpace(proxy)
	secretMu.Unlock()
}

func getSecrets() (string, string) {
	secretMu.Lock()
	defer secretMu.Unlock()
	return secretKey, secretProxy
}

func registerShotHotkeys(shot, translate string) FlagResult {
	a, b, note := reviewHotkeys(shot, translate)
	unregisterHotKey.Call(msgHwnd, 1)
	unregisterHotKey.Call(msgHwnd, 2)
	conflict := strings.Contains(note, "冲突")
	var extra []string
	if !conflict && a.VK != 0 && call(registerHotKey, msgHwnd, 1, uintptr(a.Mods), uintptr(a.VK)) == 0 {
		extra = append(extra, "截图快捷键 "+a.Norm+" 已被其他程序占用")
	}
	if !conflict && b.VK != 0 && call(registerHotKey, msgHwnd, 2, uintptr(b.Mods), uintptr(b.VK)) == 0 {
		extra = append(extra, "截图翻译快捷键 "+b.Norm+" 已被其他程序占用")
	}
	if len(extra) > 0 {
		if note != "" {
			note += "\n"
		}
		note += strings.Join(extra, "\n")
	}
	if note != "" {
		log.Printf("screenshot hotkeys: %s", note)
	}
	rememberWidgetHotkeys(a.Norm, b.Norm)
	return FlagResult{true, note}
}

func onHotkey(id uintptr) {
	if liveOverlay != nil {
		closeOverlay()
		return
	}
	switch id {
	case 1:
		beginCapture(modeShot)
	case 2:
		beginCapture(modeTranslate)
	}
}

func beginCapture(mode int) {
	if liveOverlay != nil {
		hwnd := liveOverlay.hwnd
		releaseCapture.Call()
		destroyWindow.Call(hwnd)
	}
	hideFloatWidget()
	hideResultForCapture()
	hideBusyForCapture()
	ov, err := captureVirtual()
	if err != nil {
		log.Printf("screenshot: %v", err)
		showFloatWidget()
		showPopup(80, 80, "", "", err.Error(), "")
		return
	}
	ov.mode = mode
	hwnd := createShotWindow(wsExTopmost|wsExTool, wsPopup|wsVisible, ov.vx, ov.vy, ov.vw, ov.vh)
	if hwnd == 0 {
		log.Printf("screenshot window: %v", windows.GetLastError())
		freeOverlayGDI(ov)
		showFloatWidget()
		showPopup(80, 80, "", "", "截图窗口创建失败", "")
		return
	}
	ov.hwnd = hwnd
	liveOverlay = ov
	windowKind[hwnd] = kindOverlay
	showWindow.Call(hwnd, swShow)
	setWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShow)
	setForegroundWindow.Call(hwnd)
	setFocus.Call(hwnd)
}

func createShotWindow(ex, style uintptr, x, y, w, h int32) uintptr {
	return call(createWindowEx,
		ex,
		uintptr(unsafe.Pointer(&shotClassName[0])),
		0,
		style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, shotInstance, 0,
	)
}

func closeOverlay() {
	if liveOverlay == nil {
		showFloatWidget()
		return
	}
	hwnd := liveOverlay.hwnd
	releaseCapture.Call()
	destroyWindow.Call(hwnd)
	showFloatWidget()
}

func shotWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("screenshot window: %v", rec)
		}
	}()
	switch msg {
	case wmEraseBkgnd:
		return 1
	case wmMouseActivate:
		if liveWidget != nil && liveWidget.hwnd == hwnd {
			return maNoActivate
		}
	case wmDestroy:
		onDestroy(hwnd)
		return 0
	case wmPaint:
		paintWindow(hwnd)
		return 0
	case wmKeyDown:
		if wParam == vkEscape {
			onEscape(hwnd)
			return 0
		}
	case wmLButtonDown:
		onLeftDown(hwnd)
		return 0
	case wmMouseMove:
		onMouseMove(hwnd)
		return 0
	case wmLButtonUp:
		onLeftUp(hwnd)
		return 0
	case wmLButtonDblClk:
		if pins[hwnd] != nil {
			closePin(hwnd)
			return 0
		}
	case wmRButtonUp:
		if p := pins[hwnd]; p != nil {
			pinMenu(p)
			return 0
		}
		if liveOverlay != nil && liveOverlay.hwnd == hwnd && liveOverlay.phase == phaseSelecting {
			closeOverlay()
			return 0
		}
	case wmMouseWheel:
		onWheel(hwnd, wParam)
		return 0
	}
	_ = lParam
	return call(defWindowProc, hwnd, msg, wParam, lParam)
}

func onDestroy(hwnd uintptr) {
	if liveOverlay != nil && liveOverlay.hwnd == hwnd {
		freeOverlayGDI(liveOverlay)
		liveOverlay = nil
	}
	if p := pins[hwnd]; p != nil {
		if p.bmp != 0 {
			deleteObject.Call(p.bmp)
		}
		delete(pins, hwnd)
	}
	if livePopup != nil && livePopup.hwnd == hwnd {
		livePopup = nil
	}
	if busyHwnd == hwnd {
		busyHwnd = 0
	}
	if liveResult != nil && liveResult.hwnd == hwnd {
		releaseResult(liveResult)
		liveResult = nil
	}
	if liveWidget != nil && liveWidget.hwnd == hwnd {
		if liveWidget.bull != 0 {
			deleteObject.Call(liveWidget.bull)
			liveWidget.bull = 0
		}
		liveWidget = nil
	}
	delete(windowKind, hwnd)
}

func onEscape(hwnd uintptr) {
	if liveWidget != nil && liveWidget.hwnd == hwnd {
		return
	}
	if liveResult != nil && liveResult.hwnd == hwnd {
		destroyWindow.Call(hwnd)
		return
	}
	if liveOverlay != nil && liveOverlay.hwnd == hwnd {
		closeOverlay()
		return
	}
	if pins[hwnd] != nil {
		closePin(hwnd)
		return
	}
	if livePopup != nil && livePopup.hwnd == hwnd {
		destroyWindow.Call(hwnd)
		return
	}
	if busyHwnd == hwnd {
		cancelJob()
	}
}

func onLeftDown(hwnd uintptr) {
	if liveWidget != nil && liveWidget.hwnd == hwnd {
		onWidgetLeftDown(hwnd)
		return
	}
	if liveResult != nil && liveResult.hwnd == hwnd {
		onResultLeftDown(hwnd)
		return
	}
	if liveOverlay != nil && liveOverlay.hwnd == hwnd {
		x, y := clientCursor(hwnd)
		ov := liveOverlay
		if ov.phase == phaseToolbar {
			if id := hitButton(ov, x, y); id != 0 {
				runToolbar(id)
				return
			}
			sx, sy, sw, sh := normRect(ov.x0, ov.y0, ov.x1, ov.y1)
			if x < sx || y < sy || x >= sx+sw || y >= sy+sh {
				closeOverlay()
			}
			return
		}
		ov.dragging = true
		ov.x0, ov.y0, ov.x1, ov.y1 = x, y, x, y
		setCapture.Call(hwnd)
		return
	}
	if p := pins[hwnd]; p != nil {
		setFocus.Call(hwnd)
		var pt point
		getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		var rc rect
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		p.drag = true
		p.offX = pt.X - rc.Left
		p.offY = pt.Y - rc.Top
		setCapture.Call(hwnd)
		return
	}
	if livePopup != nil && livePopup.hwnd == hwnd {
		var cr rect
		getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
		x, y := clientCursor(hwnd)
		c := closeBox(cr.Right)
		if x >= c.Left && y >= c.Top && x < c.Right && y < c.Bottom {
			destroyWindow.Call(hwnd)
		}
		return
	}
	if busyHwnd == hwnd {
		cancelJob()
	}
}

func onMouseMove(hwnd uintptr) {
	if liveWidget != nil && liveWidget.hwnd == hwnd {
		onWidgetMouseMove(hwnd)
		return
	}
	if liveResult != nil && liveResult.hwnd == hwnd {
		onResultMouseMove(hwnd)
		return
	}
	if liveOverlay != nil && liveOverlay.hwnd == hwnd && liveOverlay.dragging {
		x, y := clientCursor(hwnd)
		ov := liveOverlay
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x > ov.vw {
			x = ov.vw
		}
		if y > ov.vh {
			y = ov.vh
		}
		ov.x1, ov.y1 = x, y
		invalidateRect.Call(hwnd, 0, 0)
		return
	}
	if p := pins[hwnd]; p != nil && p.drag {
		var pt point
		getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		movePin(p, pt.X-p.offX, pt.Y-p.offY)
	}
}

func onLeftUp(hwnd uintptr) {
	if liveWidget != nil && liveWidget.hwnd == hwnd {
		onWidgetLeftUp(hwnd)
		return
	}
	if liveResult != nil && liveResult.hwnd == hwnd {
		onResultLeftUp(hwnd)
		return
	}
	if liveOverlay != nil && liveOverlay.hwnd == hwnd && liveOverlay.dragging {
		x, y := clientCursor(hwnd)
		liveOverlay.x1, liveOverlay.y1 = x, y
		finishSelection()
		return
	}
	if p := pins[hwnd]; p != nil && p.drag {
		p.drag = false
		releaseCapture.Call()
		var pt point
		getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		movePin(p, pt.X-p.offX, pt.Y-p.offY)
	}
}

func onWheel(hwnd, wParam uintptr) {
	p := pins[hwnd]
	if p == nil {
		return
	}
	lo := uint32(wParam)
	delta := int16(lo >> 16)
	if lo&mkControl != 0 {
		fadePin(p, delta)
		return
	}
	zoomPin(p, delta)
}

func clientCursor(hwnd uintptr) (int32, int32) {
	var pt point
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	var rc rect
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	return pt.X - rc.Left, pt.Y - rc.Top
}

func normRect(x0, y0, x1, y1 int32) (int32, int32, int32, int32) {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	return x0, y0, x1 - x0, y1 - y0
}

func finishSelection() {
	ov := liveOverlay
	if ov == nil {
		return
	}
	ov.dragging = false
	releaseCapture.Call()
	x, y, w, h := normRect(ov.x0, ov.y0, ov.x1, ov.y1)
	if w < 8 || h < 8 {
		closeOverlay()
		return
	}
	if ov.mode == modeTranslate {
		px, cw, ch := cropBGRA(ov.px, ov.vw, ov.vh, x, y, w, h)
		pngBytes, err := encodePNG(px, cw, ch)
		sx, sy := ov.vx+x, ov.vy+y+h+8
		closeOverlay()
		if err != nil {
			showPopup(sx, sy, "", "", err.Error(), "")
			return
		}
		id := nextJob()
		showBusy(sx, sy)
		startRecognize(pngBytes, sx, sy, id)
		return
	}
	ov.phase = phaseToolbar
	layoutButtons(ov)
	invalidateRect.Call(ov.hwnd, 0, 0)
}

func runToolbar(id int) {
	ov := liveOverlay
	if ov == nil {
		return
	}
	x, y, w, h := normRect(ov.x0, ov.y0, ov.x1, ov.y1)
	px, cw, ch := cropBGRA(ov.px, ov.vw, ov.vh, x, y, w, h)
	sx, sy := ov.vx+x, ov.vy+y
	switch id {
	case 1:
		err := copyImage(px, cw, ch)
		closeOverlay()
		if err != nil {
			showPopup(sx, sy, "", "", err.Error(), "")
		}
	case 2:
		err := saveImage(ov.hwnd, px, cw, ch)
		if errors.Is(err, errSaveCancelled) {
			return
		}
		closeOverlay()
		if err != nil {
			showPopup(sx, sy, "", "", err.Error(), "")
		}
	case 3:
		closeOverlay()
		createPin(px, cw, ch, sx, sy)
	}
}

func hitButton(ov *overlay, x, y int32) int {
	for _, b := range ov.buttons {
		if x >= b.x && y >= b.y && x < b.x+b.w && y < b.y+b.h {
			return b.id
		}
	}
	return 0
}

func layoutButtons(ov *overlay) {
	x, y, w, h := normRect(ov.x0, ov.y0, ov.x1, ov.y1)
	labels := []struct {
		id   int
		text string
	}{{1, "复制"}, {2, "保存"}, {3, "贴住"}}
	bw, bh, gap := int32(76), int32(30), int32(8)
	total := int32(len(labels))*bw + gap*int32(len(labels)-1)
	left := x + w - total
	top := y + h + 10
	if left < 8 {
		left = 8
	}
	if left+total > ov.vw-8 {
		left = ov.vw - 8 - total
	}
	if top+bh > ov.vh-8 {
		top = y - bh - 10
	}
	if top < 8 {
		top = 8
	}
	ov.buttons = ov.buttons[:0]
	for i, item := range labels {
		ov.buttons = append(ov.buttons, shotButton{
			label: item.text,
			id:    item.id,
			x:     left + int32(i)*(bw+gap),
			y:     top,
			w:     bw,
			h:     bh,
		})
	}
}

func createPin(px []byte, w, h, x, y int32) {
	if len(px) == 0 || w < 1 || h < 1 {
		return
	}
	owned := append([]byte(nil), px...)
	bmp := bitmapFromBGRA(owned, w, h)
	if bmp == 0 {
		showPopup(x, y, "", "", "贴住失败", "")
		return
	}
	hwnd := createShotWindow(wsExTopmost|wsExTool|wsExLayered, wsPopup|wsVisible, x, y, w, h)
	if hwnd == 0 {
		deleteObject.Call(bmp)
		showPopup(x, y, "", "", "贴住失败", "")
		return
	}
	setLayeredWindowAttributes.Call(hwnd, 0, 255, lwaAlpha)
	pins[hwnd] = &pin{hwnd: hwnd, bmp: bmp, px: owned, imgW: w, imgH: h, zoom: 1, alpha: 255}
	windowKind[hwnd] = kindPin
	showWindow.Call(hwnd, swShow)
	setWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShow)
}

func closePin(hwnd uintptr) {
	releaseCapture.Call()
	destroyWindow.Call(hwnd)
}

func movePin(p *pin, x, y int32) {
	var rc rect
	getWindowRect.Call(p.hwnd, uintptr(unsafe.Pointer(&rc)))
	w, h := rc.Right-rc.Left, rc.Bottom-rc.Top
	x, y = magnet(p.hwnd, x, y, w, h)
	setWindowPos.Call(p.hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
}

func magnet(hwnd uintptr, x, y, w, h int32) (int32, int32) {
	var mi monitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	mon := call(monitorFromWindow, hwnd, monitorNearest)
	if mon == 0 || call(getMonitorInfo, mon, uintptr(unsafe.Pointer(&mi))) == 0 {
		return x, y
	}
	const gap int32 = 16
	if abs32(x-mi.RcWork[0]) <= gap {
		x = mi.RcWork[0]
	}
	if abs32(y-mi.RcWork[1]) <= gap {
		y = mi.RcWork[1]
	}
	if abs32((x+w)-mi.RcWork[2]) <= gap {
		x = mi.RcWork[2] - w
	}
	if abs32((y+h)-mi.RcWork[3]) <= gap {
		y = mi.RcWork[3] - h
	}
	return x, y
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func zoomPin(p *pin, delta int16) {
	if delta > 0 {
		p.zoom *= 1.12
	} else if delta < 0 {
		p.zoom /= 1.12
	}
	if p.zoom < 0.25 {
		p.zoom = 0.25
	}
	if p.zoom > 8 {
		p.zoom = 8
	}
	if p.zoom > 0.97 && p.zoom < 1.03 {
		p.zoom = 1
	}
	var rc rect
	getWindowRect.Call(p.hwnd, uintptr(unsafe.Pointer(&rc)))
	cx := rc.Left + (rc.Right-rc.Left)/2
	cy := rc.Top + (rc.Bottom-rc.Top)/2
	newW := int32(math.Round(float64(p.imgW) * p.zoom))
	newH := int32(math.Round(float64(p.imgH) * p.zoom))
	if newW < 24 {
		newW = 24
	}
	if newH < 24 {
		newH = 24
	}
	nx, ny := cx-newW/2, cy-newH/2
	setWindowPos.Call(p.hwnd, 0, uintptr(nx), uintptr(ny), uintptr(newW), uintptr(newH), swpNoZOrder|swpNoActivate)
	invalidateRect.Call(p.hwnd, 0, 0)
}

func fadePin(p *pin, delta int16) {
	a := int(p.alpha)
	if delta > 0 {
		a += 18
	} else if delta < 0 {
		a -= 18
	}
	if a < 28 {
		a = 28
	}
	if a > 255 {
		a = 255
	}
	p.alpha = byte(a)
	setLayeredWindowAttributes.Call(p.hwnd, 0, uintptr(p.alpha), lwaAlpha)
}

func pinMenu(p *pin) {
	setForegroundWindow.Call(p.hwnd)
	menu := call(createPopupMenu)
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	appendItem(menu, 1, "复制")
	appendItem(menu, 2, "保存")
	appendItem(menu, 3, "翻译")
	var pt point
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	cmd := call(trackPopupMenu, menu, tpmReturnCmd|tpmNoNotify, uintptr(pt.X), uintptr(pt.Y), 0, p.hwnd, 0)
	switch cmd {
	case 1:
		if err := copyImage(p.px, p.imgW, p.imgH); err != nil {
			showPopup(pt.X, pt.Y, "", "", err.Error(), "")
		}
	case 2:
		if err := saveImage(p.hwnd, p.px, p.imgW, p.imgH); err != nil && !errors.Is(err, errSaveCancelled) {
			showPopup(pt.X, pt.Y, "", "", err.Error(), "")
		}
	case 3:
		pngBytes, err := encodePNG(p.px, p.imgW, p.imgH)
		if err != nil {
			showPopup(pt.X, pt.Y, "", "", err.Error(), "")
			return
		}
		var rc rect
		getWindowRect.Call(p.hwnd, uintptr(unsafe.Pointer(&rc)))
		id := nextJob()
		showBusy(rc.Left, rc.Bottom+8)
		startRecognize(pngBytes, rc.Left, rc.Bottom+8, id)
	}
}

func appendItem(menu, id uintptr, label string) {
	text, err := windows.UTF16PtrFromString(label)
	if err != nil {
		return
	}
	appendMenu.Call(menu, 0, id, uintptr(unsafe.Pointer(text)))
}

func nextJob() int {
	currentJob++
	return currentJob
}

func cancelJob() {
	currentJob++
	if busyHwnd != 0 {
		destroyWindow.Call(busyHwnd)
	}
}

func showBusy(x, y int32) {
	if busyHwnd != 0 {
		destroyWindow.Call(busyHwnd)
	}
	const w, h int32 = 260, 72
	x, y = clampToScreen(x, y, w, h)
	hwnd := createShotWindow(wsExTopmost|wsExTool, wsPopup|wsVisible, x, y, w, h)
	if hwnd == 0 {
		return
	}
	busyHwnd = hwnd
	windowKind[hwnd] = kindBusy
	showWindow.Call(hwnd, swShow)
	setWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShow)
	setForegroundWindow.Call(hwnd)
	setFocus.Call(hwnd)
}

func startRecognize(png []byte, x, y int32, id int) {
	go func() {
		original, err := recognizeShotPNG(png)
		if err != nil {
			postResult(shotResult{id: id, errText: err.Error(), x: x, y: y, png: png})
			return
		}
		key, proxy := getSecrets()
		zh, provider, err := translateScreenshotText(original, key, proxy)
		if err != nil {
			postResult(shotResult{id: id, original: original, errText: err.Error(), x: x, y: y, png: png})
			return
		}
		postResult(shotResult{id: id, original: original, chinese: zh, provider: provider, x: x, y: y, png: png})
	}()
}

func postResult(r shotResult) {
	resultMu.Lock()
	resultQ = append(resultQ, r)
	resultMu.Unlock()
	if msgHwnd != 0 {
		postMessage.Call(msgHwnd, wmShotResult, 0, 0)
	}
}

func drainResults() {
	resultMu.Lock()
	batch := resultQ
	resultQ = nil
	resultMu.Unlock()
	for _, item := range batch {
		if item.id != currentJob {
			continue
		}
		if busyHwnd != 0 {
			destroyWindow.Call(busyHwnd)
		}
		showShotResult(item)
	}
}

func showPopup(x, y int32, original, chinese, errText, provider string) {
	if livePopup != nil {
		destroyWindow.Call(livePopup.hwnd)
	}
	note := providerNote(provider)
	if errText != "" {
		if strings.TrimSpace(chinese) == "" {
			chinese = errText
		} else {
			note = errText
		}
	}
	body := popupBody(original, chinese)
	_, bodyH := measureBody(body, 460)
	w := int32(520)
	h := bodyH + 88
	if h < 160 {
		h = 160
	}
	if h > 640 {
		h = 640
	}
	x, y = clampToScreen(x, y, w, h)
	hwnd := createShotWindow(wsExTopmost|wsExTool, wsPopup|wsVisible, x, y, w, h)
	if hwnd == 0 {
		return
	}
	livePopup = &textPopup{hwnd: hwnd, title: "截图翻译", original: original, chinese: chinese, note: note}
	windowKind[hwnd] = kindPopup
	showWindow.Call(hwnd, swShow)
	setWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShow)
	setForegroundWindow.Call(hwnd)
	setFocus.Call(hwnd)
}

func popupBody(original, chinese string) string {
	if strings.TrimSpace(original) == "" {
		return chinese
	}
	var b strings.Builder
	b.WriteString("原文\n")
	b.WriteString(original)
	if strings.TrimSpace(chinese) != "" {
		b.WriteString("\n\n译文\n")
		b.WriteString(chinese)
	}
	return b.String()
}

func (p *textPopup) body() string {
	if p == nil {
		return ""
	}
	return popupBody(p.original, p.chinese)
}

func clampToScreen(x, y, w, h int32) (int32, int32) {
	vx := int32(call(getSystemMetrics, smXVirtualScreen))
	vy := int32(call(getSystemMetrics, smYVirtualScreen))
	vw := int32(call(getSystemMetrics, smCXVirtualScreen))
	vh := int32(call(getSystemMetrics, smCYVirtualScreen))
	if vw <= 0 || vh <= 0 {
		return x, y
	}
	if x+w > vx+vw-8 {
		x = vx + vw - w - 8
	}
	if y+h > vy+vh-8 {
		y = vy + vh - h - 8
	}
	if x < vx+8 {
		x = vx + 8
	}
	if y < vy+8 {
		y = vy + 8
	}
	return x, y
}

func paintWindow(hwnd uintptr) {
	var ps paintStruct
	hdc := call(beginPaint, hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer endPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	switch windowKind[hwnd] {
	case kindOverlay:
		paintOverlay(hdc)
	case kindPin:
		paintPin(hdc, pins[hwnd])
	case kindPopup:
		paintPopup(hdc, livePopup)
	case kindBusy:
		paintBusy(hdc, hwnd)
	case kindWidget:
		paintFloatWidget(hdc)
	case kindResult:
		paintTranslateResult(hdc)
	}
}

func paintOverlay(hdc uintptr) {
	ov := liveOverlay
	if ov == nil || ov.hdcMem == 0 {
		return
	}
	bitBlt.Call(hdc, 0, 0, uintptr(ov.vw), uintptr(ov.vh), ov.hdcMem, 0, 0, srcCopy)
	x, y, w, h := normRect(ov.x0, ov.y0, ov.x1, ov.y1)
	if (ov.dragging || ov.phase == phaseToolbar) && w > 0 && h > 0 {
		dimScreen(hdc, ov.vw, ov.vh)
		bitBlt.Call(hdc, uintptr(x), uintptr(y), uintptr(w), uintptr(h), ov.hdcMem, uintptr(x), uintptr(y), srcCopy)
		strokeRect(hdc, x, y, w, h, colorRef(232, 213, 163), 2)
	}
	if ov.phase == phaseToolbar {
		for _, b := range ov.buttons {
			paintButton(hdc, b)
		}
	}
	paintHint(hdc, ov)
}

func dimScreen(hdc uintptr, w, h int32) {
	ensureDim(hdc)
	if dimDC == 0 {
		return
	}
	alphaBlend.Call(hdc, 0, 0, uintptr(w), uintptr(h), dimDC, 0, 0, 1, 1, uintptr(120)<<16)
}

func ensureDim(hdc uintptr) {
	if dimDC != 0 {
		return
	}
	dimDC = call(createCompatibleDC, hdc)
	dimBmp = call(createCompatibleBitmap, hdc, 1, 1)
	if dimDC == 0 || dimBmp == 0 {
		return
	}
	selectObject.Call(dimDC, dimBmp)
	patBlt.Call(dimDC, 0, 0, 1, 1, blackness)
}

func paintHint(hdc uintptr, ov *overlay) {
	text := "拖动框选，Esc 取消"
	if ov.mode == modeTranslate {
		text = "拖动框选英文，松开后翻译，Esc 取消"
	}
	bw, bh := int32(420), int32(36)
	x := (ov.vw - bw) / 2
	if x < 8 {
		x = 8
	}
	fillBox(hdc, x, 16, bw, bh, colorRef(6, 20, 40))
	strokeRect(hdc, x, 16, bw, bh, colorRef(232, 213, 163), 1)
	drawTextBox(hdc, text, x, 16, bw, bh, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(159, 214, 255))
}

func paintButton(hdc uintptr, b shotButton) {
	fillBox(hdc, b.x, b.y, b.w, b.h, colorRef(16, 36, 60))
	strokeRect(hdc, b.x, b.y, b.w, b.h, colorRef(232, 213, 163), 1)
	drawTextBox(hdc, b.label, b.x, b.y, b.w, b.h, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(246, 241, 230))
}

func paintPin(hdc uintptr, p *pin) {
	if p == nil || p.bmp == 0 {
		return
	}
	var cr rect
	getClientRect.Call(p.hwnd, uintptr(unsafe.Pointer(&cr)))
	dw, dh := cr.Right, cr.Bottom
	mem := call(createCompatibleDC, hdc)
	if mem == 0 {
		return
	}
	old := call(selectObject, mem, p.bmp)
	if dw == p.imgW && dh == p.imgH {
		bitBlt.Call(hdc, 0, 0, uintptr(dw), uintptr(dh), mem, 0, 0, srcCopy)
	} else {
		setStretchBltMode.Call(hdc, colorOnColor)
		stretchBlt.Call(hdc, 0, 0, uintptr(dw), uintptr(dh), mem, 0, 0, uintptr(p.imgW), uintptr(p.imgH), srcCopy)
	}
	selectObject.Call(mem, old)
	deleteDC.Call(mem)
	strokeRect(hdc, 0, 0, dw, dh, colorRef(232, 213, 163), 1)
}

func paintPopup(hdc uintptr, p *textPopup) {
	if p == nil {
		return
	}
	var cr rect
	getClientRect.Call(p.hwnd, uintptr(unsafe.Pointer(&cr)))
	fillBox(hdc, 0, 0, cr.Right, cr.Bottom, colorRef(6, 20, 40))
	strokeRect(hdc, 1, 1, cr.Right-2, cr.Bottom-2, colorRef(232, 213, 163), 1)
	drawTextBox(hdc, p.title, 16, 8, cr.Right-90, 28, dtSingleLine|dtVCenter|dtNoPrefix, colorRef(232, 213, 163))
	c := closeBox(cr.Right)
	fillBox(hdc, c.Left, c.Top, c.Right-c.Left, c.Bottom-c.Top, colorRef(16, 36, 60))
	strokeRect(hdc, c.Left, c.Top, c.Right-c.Left, c.Bottom-c.Top, colorRef(159, 214, 255), 1)
	drawTextBox(hdc, "关闭", c.Left, c.Top, c.Right-c.Left, c.Bottom-c.Top, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(159, 214, 255))
	drawTextBox(hdc, p.body(), 16, 44, cr.Right-32, cr.Bottom-80, dtWordBreak|dtNoPrefix, colorRef(246, 241, 230))
	if p.note != "" {
		drawTextBox(hdc, p.note, 16, cr.Bottom-28, cr.Right-32, 20, dtSingleLine|dtNoPrefix, colorRef(159, 214, 255))
	}
}

func paintBusy(hdc uintptr, hwnd uintptr) {
	var cr rect
	getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	fillBox(hdc, 0, 0, cr.Right, cr.Bottom, colorRef(6, 20, 40))
	strokeRect(hdc, 1, 1, cr.Right-2, cr.Bottom-2, colorRef(232, 213, 163), 1)
	drawTextBox(hdc, "正在识别并翻译…", 12, 0, cr.Right-24, cr.Bottom, dtCenter|dtVCenter|dtSingleLine|dtNoPrefix, colorRef(246, 241, 230))
}

func closeBox(width int32) rect {
	return rect{Left: width - 68, Top: 10, Right: width - 12, Bottom: 34}
}

func fillBox(hdc uintptr, x, y, w, h int32, color uintptr) {
	if w <= 0 || h <= 0 {
		return
	}
	brush := call(createSolidBrush, color)
	if brush == 0 {
		return
	}
	r := rect{Left: x, Top: y, Right: x + w, Bottom: y + h}
	fillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
	deleteObject.Call(brush)
}

func strokeRect(hdc uintptr, x, y, w, h int32, color uintptr, width int32) {
	if w <= 0 || h <= 0 {
		return
	}
	pen := call(createPen, 0, uintptr(width), color)
	if pen == 0 {
		return
	}
	oldPen := call(selectObject, hdc, pen)
	oldBrush := call(selectObject, hdc, call(getStockObject, nullBrush))
	rectangle.Call(hdc, uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+h))
	selectObject.Call(hdc, oldBrush)
	selectObject.Call(hdc, oldPen)
	deleteObject.Call(pen)
}

func drawTextBox(hdc uintptr, text string, x, y, w, h int32, format uintptr, color uintptr) {
	if text == "" || w <= 0 || h <= 0 {
		return
	}
	font := uiFont()
	var old uintptr
	if font != 0 {
		old = call(selectObject, hdc, font)
	}
	setBkMode.Call(hdc, 1)
	setTextColor.Call(hdc, color)
	encoded, err := windows.UTF16PtrFromString(text)
	if err == nil {
		r := rect{Left: x, Top: y, Right: x + w, Bottom: y + h}
		drawTextW.Call(hdc, uintptr(unsafe.Pointer(encoded)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), format)
	}
	if font != 0 {
		selectObject.Call(hdc, old)
	}
}

func uiFont() uintptr {
	if fontHandle != 0 {
		return fontHandle
	}
	name, err := windows.UTF16PtrFromString("Microsoft YaHei")
	if err != nil {
		return 0
	}
	height := int32(-18)
	fontHandle = call(createFont, uintptr(height), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(name)))
	return fontHandle
}

func measureBody(text string, width int32) (int32, int32) {
	if strings.TrimSpace(text) == "" {
		return width, 48
	}
	screen := call(getDC, 0)
	if screen == 0 {
		return width, 180
	}
	defer releaseDC.Call(0, screen)
	mem := call(createCompatibleDC, screen)
	if mem == 0 {
		return width, 180
	}
	defer deleteDC.Call(mem)
	font := uiFont()
	if font != 0 {
		old := call(selectObject, mem, font)
		defer selectObject.Call(mem, old)
	}
	encoded, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return width, 180
	}
	r := rect{Right: width}
	drawTextW.Call(mem, uintptr(unsafe.Pointer(encoded)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), dtWordBreak|dtNoPrefix|dtCalcRect)
	if r.Bottom < 40 {
		r.Bottom = 40
	}
	return width, r.Bottom
}
