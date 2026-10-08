package bridge

import (
	"encoding/base64"
	"testing"
)

func TestArgsOpenSecurity(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(
		`{"action":"DEFAULT_ACTION","payload":{"id":"dhagn-security-alert","title":"VPN 安全检查发现问题，点击查看"}}`,
	))
	if !ArgsOpenSecurity([]string{`"` + payload + `"`}) {
		t.Fatal("encoded toast activation should open the check")
	}
	if !ArgsOpenSecurity([]string{SecurityNoticeID}) {
		t.Fatal("plain id should open the check")
	}
	if ArgsOpenSecurity([]string{"gui://install-config/?url=https://example.invalid"}) {
		t.Fatal("ordinary launch args should stay on the import path")
	}
	if ArgsOpenSecurity(nil) {
		t.Fatal("empty args are not a security click")
	}
}
