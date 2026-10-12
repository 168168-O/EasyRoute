//go:build windows

package bridge

import (
	"log"
	"strings"

	"github.com/zn-chen/sysocr"
)

// recognizeShotPNG uses Windows.Media.Ocr through sysocr.
// sysocr is pure Go on Windows (no CGO), which matches this app's Windows build.
// "en" is passed as the language hint. This release of sysocr still asks Windows
// for the user-profile OCR languages; English text is read when that pack can
// recognize Latin script.
func recognizeShotPNG(png []byte) (string, error) {
	if len(png) == 0 {
		return "", errNoShotText
	}
	result, err := sysocr.Recognize(sysocr.Options{
		Input:     sysocr.Input{Data: png},
		Languages: []string{"en"},
	})
	if err != nil {
		log.Printf("screenshot ocr: %v", err)
		if strings.Contains(strings.ToLower(err.Error()), "language") {
			return "", errShotOCRLang
		}
		return "", errShotOCR
	}
	if result == nil {
		return "", errNoShotText
	}
	text := strings.TrimSpace(result.Text)
	if text == "" {
		return "", errNoShotText
	}
	return text, nil
}
