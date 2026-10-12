//go:build windows

package bridge

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	smXVirtualScreen   = 76
	smYVirtualScreen   = 77
	smCXVirtualScreen  = 78
	smCYVirtualScreen  = 79
	srcCopy            = 0x00CC0020
	captureBlt         = 0x40000000
	blackness          = 0x00000042
	biRGB              = 0
	dibRGBColors       = 0
	cfDIB              = 8
	gmemMoveable       = 0x0002
	ofnExplorer        = 0x00080000
	ofnPathMustExist   = 0x00000800
	ofnOverwritePrompt = 0x00000002
	ofnNoChangeDir     = 0x00000008
)

func captureVirtual() (*overlay, error) {
	vx := int32(call(getSystemMetrics, smXVirtualScreen))
	vy := int32(call(getSystemMetrics, smYVirtualScreen))
	vw := int32(call(getSystemMetrics, smCXVirtualScreen))
	vh := int32(call(getSystemMetrics, smCYVirtualScreen))
	if vw < 16 || vh < 16 || int64(vw)*int64(vh) > 24_000_000 {
		return nil, errors.New("读不到可用的屏幕尺寸")
	}
	screen := call(getDC, 0)
	if screen == 0 {
		return nil, errors.New("读不到屏幕")
	}
	defer releaseDC.Call(0, screen)
	mem := call(createCompatibleDC, screen)
	bmp := call(createCompatibleBitmap, screen, uintptr(vw), uintptr(vh))
	if mem == 0 || bmp == 0 {
		if bmp != 0 {
			deleteObject.Call(bmp)
		}
		if mem != 0 {
			deleteDC.Call(mem)
		}
		return nil, errors.New("创建截图失败")
	}
	old := call(selectObject, mem, bmp)
	if call(bitBlt, mem, 0, 0, uintptr(vw), uintptr(vh), screen, uintptr(vx), uintptr(vy), srcCopy|captureBlt) == 0 {
		selectObject.Call(mem, old)
		deleteObject.Call(bmp)
		deleteDC.Call(mem)
		return nil, errors.New("截取屏幕失败")
	}
	hdr := bitmapInfoHeader{
		BiSize:      40,
		BiWidth:     vw,
		BiHeight:    -vh,
		BiPlanes:    1,
		BiBitCount:  32,
		BiSizeImage: uint32(vw * vh * 4),
	}
	px := make([]byte, int(vw)*int(vh)*4)
	lines := call(getDIBits, mem, bmp, 0, uintptr(vh), uintptr(unsafe.Pointer(&px[0])), uintptr(unsafe.Pointer(&hdr)), dibRGBColors)
	if lines == 0 {
		selectObject.Call(mem, old)
		deleteObject.Call(bmp)
		deleteDC.Call(mem)
		return nil, errors.New("读取屏幕像素失败")
	}
	return &overlay{
		hdcMem: mem,
		bmp:    bmp,
		oldBmp: old,
		vx:     vx,
		vy:     vy,
		vw:     vw,
		vh:     vh,
		px:     px,
	}, nil
}

func freeOverlayGDI(ov *overlay) {
	if ov == nil {
		return
	}
	if ov.hdcMem != 0 && ov.oldBmp != 0 {
		selectObject.Call(ov.hdcMem, ov.oldBmp)
	}
	if ov.bmp != 0 {
		deleteObject.Call(ov.bmp)
		ov.bmp = 0
	}
	if ov.hdcMem != 0 {
		deleteDC.Call(ov.hdcMem)
		ov.hdcMem = 0
	}
	ov.px = nil
}

func cropBGRA(src []byte, sw, sh, x, y, w, h int32) ([]byte, int32, int32) {
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > sw {
		w = sw - x
	}
	if y+h > sh {
		h = sh - y
	}
	if w <= 0 || h <= 0 || len(src) < int(sw*sh*4) {
		return nil, 0, 0
	}
	dst := make([]byte, int(w*h*4))
	sStride := int(sw * 4)
	dStride := int(w * 4)
	for row := int32(0); row < h; row++ {
		so := int(y+row)*sStride + int(x)*4
		copy(dst[int(row)*dStride:int(row)*dStride+dStride], src[so:so+dStride])
	}
	return dst, w, h
}

func encodePNG(px []byte, w, h int32) ([]byte, error) {
	if w <= 0 || h <= 0 || len(px) < int(w*h*4) {
		return nil, errors.New("没有可以保存的图片")
	}
	img := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	n := int(w * h)
	for i := 0; i < n; i++ {
		img.Pix[i*4+0] = px[i*4+2]
		img.Pix[i*4+1] = px[i*4+1]
		img.Pix[i*4+2] = px[i*4+0]
		img.Pix[i*4+3] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func bitmapFromBGRA(px []byte, w, h int32) uintptr {
	if w <= 0 || h <= 0 || len(px) < int(w*h*4) {
		return 0
	}
	screen := call(getDC, 0)
	if screen == 0 {
		return 0
	}
	defer releaseDC.Call(0, screen)
	mem := call(createCompatibleDC, screen)
	if mem == 0 {
		return 0
	}
	defer deleteDC.Call(mem)
	bmp := call(createCompatibleBitmap, screen, uintptr(w), uintptr(h))
	if bmp == 0 {
		return 0
	}
	flipped := make([]byte, len(px))
	stride := int(w * 4)
	for y := int32(0); y < h; y++ {
		copy(flipped[int(y)*stride:], px[int(h-1-y)*stride:int(h-1-y)*stride+stride])
	}
	hdr := bitmapInfoHeader{
		BiSize: 40, BiWidth: w, BiHeight: h, BiPlanes: 1, BiBitCount: 32,
		BiCompression: biRGB, BiSizeImage: uint32(w * h * 4),
	}
	if call(setDIBits, mem, bmp, 0, uintptr(h), uintptr(unsafe.Pointer(&flipped[0])), uintptr(unsafe.Pointer(&hdr)), dibRGBColors) == 0 {
		deleteObject.Call(bmp)
		return 0
	}
	return bmp
}

func copyImage(px []byte, w, h int32) error {
	if w <= 0 || h <= 0 || len(px) < int(w*h*4) {
		return errors.New("没有可以复制的图片")
	}
	hdr := bitmapInfoHeader{
		BiSize: 40, BiWidth: w, BiHeight: h, BiPlanes: 1, BiBitCount: 32,
		BiCompression: biRGB, BiSizeImage: uint32(w * h * 4),
	}
	stride := int(w * 4)
	total := 40 + int(w*h*4)
	hglob := call(globalAlloc, gmemMoveable, uintptr(total))
	if hglob == 0 {
		return errors.New("复制失败")
	}
	ptr := call(globalLock, hglob)
	if ptr == 0 {
		globalFree.Call(hglob)
		return errors.New("复制失败")
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), total)
	*(*bitmapInfoHeader)(unsafe.Pointer(&dst[0])) = hdr
	for y := int32(0); y < h; y++ {
		so := int(h-1-y) * stride
		do := 40 + int(y)*stride
		copy(dst[do:do+stride], px[so:so+stride])
	}
	globalUnlock.Call(hglob)
	if call(openClipboard, 0) == 0 {
		globalFree.Call(hglob)
		return errors.New("复制失败：剪贴板被占用")
	}
	emptyClipboard.Call()
	if call(setClipboardData, cfDIB, hglob) == 0 {
		closeClipboard.Call()
		globalFree.Call(hglob)
		return errors.New("复制失败")
	}
	closeClipboard.Call()
	return nil
}

func saveImage(owner uintptr, px []byte, w, h int32) error {
	pngBytes, err := encodePNG(px, w, h)
	if err != nil {
		return err
	}
	name := time.Now().Format("截图-20060102-150405.png")
	file := make([]uint16, 260)
	utf := windows.StringToUTF16(name)
	if len(utf) > len(file) {
		utf = utf[:len(file)-1]
		utf = append(utf, 0)
	}
	copy(file, utf)
	filter := windows.StringToUTF16("PNG 图片\x00*.png\x00\x00")
	title := windows.StringToUTF16("保存截图")
	defExt := windows.StringToUTF16("png")
	initial := windows.StringToUTF16(picturesDir())
	ofn := openFileName{
		Owner:      owner,
		Filter:     &filter[0],
		File:       &file[0],
		MaxFile:    uint32(len(file)),
		Title:      &title[0],
		Flags:      ofnExplorer | ofnPathMustExist | ofnOverwritePrompt | ofnNoChangeDir,
		DefExt:     &defExt[0],
		InitialDir: &initial[0],
	}
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	if call(getSaveFileName, uintptr(unsafe.Pointer(&ofn))) == 0 {
		if call(commDlgExtendedError) == 0 {
			return errSaveCancelled
		}
		return errors.New("保存对话框打开失败")
	}
	path := windows.UTF16ToString(file)
	if path == "" {
		return errors.New("没有选择保存位置")
	}
	if err = os.WriteFile(path, pngBytes, 0o644); err != nil {
		return errors.New("保存失败：" + err.Error())
	}
	return nil
}

func picturesDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	dir := filepath.Join(home, "Pictures")
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir
	}
	return home
}

var errSaveCancelled = errors.New("cancelled")
