package bridge

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScreenshotTranslateRouting(t *testing.T) {
	var deep, google, memory int
	oldDeep, oldGoogle, oldMemory := shotDeepSeekFn, shotGoogleFn, shotMemoryFn
	t.Cleanup(func() {
		shotDeepSeekFn, shotGoogleFn, shotMemoryFn = oldDeep, oldGoogle, oldMemory
	})
	shotDeepSeekFn = func(_ *http.Client, key, _ string) (string, error) {
		deep++
		if key == "bad" {
			return "", errors.New("no")
		}
		return "你好", nil
	}
	shotGoogleFn = func(proxy, _ string) (string, error) {
		google++
		if !strings.HasPrefix(proxy, "http://") && !strings.HasPrefix(proxy, "https://") {
			t.Fatalf("google proxy = %q", proxy)
		}
		return "谷歌译文", nil
	}
	shotMemoryFn = func(_ *http.Client, _ string) (string, error) {
		memory++
		return "记忆译文", nil
	}

	zh, provider, err := translateScreenshotText("hello", "sk-test", "")
	if err != nil || zh != "你好" || provider != "deepseek" || deep != 1 || google != 0 || memory != 0 {
		t.Fatalf("deepseek path zh=%q provider=%q err=%v counts=%d %d %d", zh, provider, err, deep, google, memory)
	}

	deep, google, memory = 0, 0, 0
	zh, provider, err = translateScreenshotText("hello", "", "http://127.0.0.1:7890")
	if err != nil || zh != "谷歌译文" || provider != "google" || deep != 0 || google != 1 || memory != 0 {
		t.Fatalf("google path zh=%q provider=%q err=%v counts=%d %d %d", zh, provider, err, deep, google, memory)
	}

	deep, google, memory = 0, 0, 0
	zh, provider, err = translateScreenshotText("hello", "", "")
	if err != nil || zh != "记忆译文" || provider != "mymemory" || deep != 0 || google != 0 || memory != 1 {
		t.Fatalf("memory path zh=%q provider=%q err=%v counts=%d %d %d", zh, provider, err, deep, google, memory)
	}

	deep, google, memory = 0, 0, 0
	zh, provider, err = translateScreenshotText("hello", "bad", "http://127.0.0.1:7890")
	if err != nil || provider != "google" || deep != 1 || google != 1 || memory != 0 {
		t.Fatalf("deepseek fallback google zh=%q provider=%q err=%v counts=%d %d %d", zh, provider, err, deep, google, memory)
	}

	deep, google, memory = 0, 0, 0
	zh, provider, err = translateScreenshotText("hello", "bad", "")
	if err != nil || provider != "mymemory" || deep != 1 || google != 0 || memory != 1 {
		t.Fatalf("deepseek fallback memory zh=%q provider=%q err=%v counts=%d %d %d", zh, provider, err, deep, google, memory)
	}

	if _, _, err = translateScreenshotText("   ", "sk-test", "http://127.0.0.1:7890"); err == nil {
		t.Fatal("empty text should fail")
	}
}

func TestCallDeepSeekAndGoogleAndMemory(t *testing.T) {
	deep := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hello") || strings.Contains(string(body), "sk-test") {
			t.Errorf("body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"你好"}}]}`))
	}))
	defer deep.Close()
	oldDeep := deepseekEndpoint
	deepseekEndpoint = deep.URL
	t.Cleanup(func() { deepseekEndpoint = oldDeep })
	got, err := callDeepSeek(deep.Client(), "sk-test", "hello")
	if err != nil || got != "你好" {
		t.Fatalf("deepseek = %q %v", got, err)
	}

	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "sl=en") || !strings.Contains(r.URL.RawQuery, "tl=zh-CN") {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[[["车站","station",null,null,1]],null,"en"]`))
	}))
	defer google.Close()
	oldGoogle := googleShotEndpoint
	googleShotEndpoint = google.URL
	t.Cleanup(func() { googleShotEndpoint = oldGoogle })
	got, err = callGoogleClient(google.Client(), "station")
	if err != nil || got != "车站" {
		t.Fatalf("google = %q %v", got, err)
	}

	memory := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "langpair=en%7Czh-CN") && !strings.Contains(r.URL.RawQuery, "langpair=en|zh-CN") {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"responseData":{"translatedText":"车站"},"responseStatus":200}`))
	}))
	defer memory.Close()
	oldMemory := myMemoryEndpoint
	myMemoryEndpoint = memory.URL
	t.Cleanup(func() { myMemoryEndpoint = oldMemory })
	got, err = callMyMemory(memory.Client(), "station")
	if err != nil || got != "车站" {
		t.Fatalf("memory = %q %v", got, err)
	}
}

func TestCallGoogleProxyRejectsDirect(t *testing.T) {
	if _, err := callGoogleProxy("", "hello"); err == nil {
		t.Fatal("empty proxy should fail")
	}
	if _, err := callGoogleProxy("socks5://127.0.0.1:1080", "hello"); err == nil {
		t.Fatal("socks proxy should fail")
	}
}

func TestChunkText(t *testing.T) {
	text := "alpha bravo charlie delta echo foxtrot golf hotel"
	parts := chunkText(text, 40)
	joined := strings.Join(parts, " ")
	if joined != text {
		t.Fatalf("joined = %q parts=%v", joined, parts)
	}
	if len(parts) < 2 {
		t.Fatalf("expected multiple chunks, got %v", parts)
	}
	for _, part := range parts {
		if len(part) > 40 {
			t.Fatalf("part too long: %q", part)
		}
	}
}
