package bridge

import (
	"context"
	"encoding/base64"
	"log"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const SecurityNoticeID = "dhagn-security-alert"
const SecurityNoticeText = "VPN 安全检查发现问题，点击查看"

// ArgsOpenSecurity reports whether a second process was started by clicking the security toast.
func ArgsOpenSecurity(args []string) bool {
	for _, arg := range args {
		if argOpensSecurity(arg) {
			return true
		}
	}
	return false
}

func argOpensSecurity(arg string) bool {
	trimmed := strings.Trim(arg, `"'`)
	if trimmed == SecurityNoticeID || strings.Contains(trimmed, SecurityNoticeID) {
		return true
	}
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), SecurityNoticeID)
}

func noticeIsSecurity(id, title, body string) bool {
	if id == SecurityNoticeID {
		return true
	}
	return title == SecurityNoticeText || body == SecurityNoticeText
}

// ListenForSecurityNotice opens the check when the user clicks the one security toast.
func ListenForSecurityNotice(ctx context.Context) {
	runtime.OnNotificationResponse(ctx, func(result runtime.NotificationResult) {
		if result.Error != nil {
			log.Printf("security notice: %v", result.Error)
			return
		}
		response := result.Response
		if !noticeIsSecurity(response.ID, response.Title, response.Body) {
			return
		}
		runtime.WindowShow(ctx)
		runtime.EventsEmit(ctx, "onSecurityNotice")
	})
}
