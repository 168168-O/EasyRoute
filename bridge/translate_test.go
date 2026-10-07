package bridge

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveLanguages(t *testing.T) {
	cases := []struct {
		name, text, target, from, to string
	}{
		{name: "english goes to chinese", text: "Hello, where is the station?", target: "auto", from: "", to: "zh-Hans"},
		{name: "other language goes to chinese", text: "Bonjour tout le monde", target: "", from: "", to: "zh-Hans"},
		{name: "chinese goes to english", text: "你好，车站怎么走？", target: "auto", from: "zh-Hans", to: "en"},
		{name: "explicit japanese", text: "Hello", target: "ja", from: "", to: "ja"},
		{name: "explicit chinese on chinese text flips source", text: "你好", target: "zh-Hans", from: "en", to: "zh-Hans"},
		{name: "explicit english on english text flips source", text: "Hello", target: "en", from: "zh-Hans", to: "en"},
		{name: "korean alias", text: "Hello", target: "ko", from: "", to: "ko"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, to := ResolveLanguages(tc.text, tc.target)
			if from != tc.from || to != tc.to {
				t.Fatalf("ResolveLanguages(%q, %q) = %q, %q; want %q, %q", tc.text, tc.target, from, to, tc.from, tc.to)
			}
		})
	}
}

func TestParseBingTranslations(t *testing.T) {
	body := []byte(`[{"translations":[{"text":"车站在哪里？","to":"zh-Hans","transliteration":{"text":"chezhan","script":"Latn"}}],"usedLLM":true,"detectedLanguage":{"language":"en"}}]`)
	text, detected, to, err := parseBingTranslations(body)
	if err != nil {
		t.Fatal(err)
	}
	if text != "车站在哪里？" || detected != "en" || to != "zh-Hans" {
		t.Fatalf("parsed %+v %s %s", text, detected, to)
	}
}

func TestParseBingPage(t *testing.T) {
	html := `<script>var params_AbusePreventionHelper = [2000000000000,"test-token"];</script><script>IG:"AABBCCDDEEFF00112233445566778899"</script><div data-iid="translator.5023"></div>`
	creds, err := parseBingPage(html, "https://www.bing.com/translator?mkt=zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if creds.token != "test-token" || creds.key != "2000000000000" || !strings.Contains(creds.postURL, "www.bing.com/ttranslatev3") || !strings.Contains(creds.postURL, "IG=AABBCCDDEEFF00112233445566778899") {
		t.Fatalf("creds = %+v", creds)
	}
}

const bingPageHTML = `<script>var params_AbusePreventionHelper = [2000000000000,"test-token"];</script><script>IG:"AABBCCDDEEFF00112233445566778899"</script><div data-iid="translator.5023"></div>`

func TestParseGoogleTranslations(t *testing.T) {
	body := []byte(`[[["你好","Hello",null,null,1],["世界","world",null,null,1]],null,"en"]`)
	text, err := parseGoogleTranslations(body)
	if err != nil {
		t.Fatal(err)
	}
	if text != "你好世界" {
		t.Fatalf("got %q", text)
	}
}

func TestTranslateBingDirect(t *testing.T) {
	var sawAuth, sawTranslate atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/translator"):
			sawAuth.Store(true)
			_, _ = io.WriteString(w, bingPageHTML)
		case strings.Contains(r.URL.Path, "/ttranslatev3"):
			sawTranslate.Store(true)
			if r.URL.Query().Get("IG") != "AABBCCDDEEFF00112233445566778899" {
				t.Errorf("IG = %q", r.URL.Query().Get("IG"))
			}
			body, _ := io.ReadAll(r.Body)
			values, _ := url.ParseQuery(string(body))
			if values.Get("to") != "zh-Hans" || values.Get("token") != "test-token" || values.Get("text") != "Hello" {
				t.Errorf("form = %v", values)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"translations":[{"text":"你好","to":"zh-Hans"}],"detectedLanguage":{"language":"en"}}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	engine := newTranslateEngine(server.Client(), nil)
	engine.bingPages = []string{server.URL + "/translator"}
	engine.googleURL = server.URL + "/google"

	result, err := engine.Translate("Hello", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "你好" || result.Provider != "bing" || result.SourceLang != "en" || result.TargetLang != "zh-Hans" {
		t.Fatalf("result = %+v", result)
	}
	if !sawAuth.Load() || !sawTranslate.Load() {
		t.Fatalf("auth=%v translate=%v", sawAuth.Load(), sawTranslate.Load())
	}
}

func TestTranslateFallsBackToGoogle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/google") {
			if r.URL.Query().Get("tl") != "en" {
				t.Errorf("tl = %q", r.URL.Query().Get("tl"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[[["Hello","你好",null,null,1]],null,"zh-CN"]`)
			return
		}
		http.Error(w, "bing down", http.StatusBadGateway)
	}))
	defer server.Close()

	engine := newTranslateEngine(server.Client(), nil)
	engine.bingPages = []string{server.URL + "/translator"}
	engine.googleURL = server.URL + "/google"

	result, err := engine.Translate("你好", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Hello" || result.Provider != "google" || result.TargetLang != "en" {
		t.Fatalf("result = %+v", result)
	}
}

type failDirect struct {
	inner http.RoundTripper
	hits  atomic.Int32
}

func (f *failDirect) RoundTrip(req *http.Request) (*http.Response, error) {
	f.hits.Add(1)
	return nil, errors.New("dial tcp: connection refused")
}

func TestTranslateUsesProxyOnlyAfterDirectFails(t *testing.T) {
	var proxyHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		if strings.HasSuffix(r.URL.Path, "/translator") {
			_, _ = io.WriteString(w, bingPageHTML)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"detectedLanguage":{"language":"en"},"translations":[{"text":"你好","to":"zh-Hans"}]}]`)
	}))
	defer server.Close()

	directFail := &failDirect{}
	direct := &http.Client{Transport: directFail, Timeout: 2 * time.Second}
	proxy := server.Client()
	engine := newTranslateEngine(direct, proxy)
	engine.bingPages = []string{server.URL + "/translator"}
	engine.googleURL = server.URL + "/google"

	result, err := engine.Translate("Hello", "zh-Hans")
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "你好" || result.Provider != "bing" {
		t.Fatalf("result = %+v", result)
	}
	if directFail.hits.Load() == 0 {
		t.Fatal("direct client was never tried")
	}
	if proxyHits.Load() == 0 {
		t.Fatal("proxy client was not used after the direct failure")
	}
}

func TestTranslateEmptyAndTotalFailure(t *testing.T) {
	engine := newTranslateEngine(directTranslateClient(), nil)
	if _, err := engine.Translate("   ", "auto"); !errors.Is(err, errTranslateEmpty) {
		t.Fatalf("empty err = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusBadGateway)
	}))
	defer server.Close()
	engine = newTranslateEngine(server.Client(), nil)
	engine.bingPages = []string{server.URL + "/translator"}
	engine.googleURL = server.URL + "/google"
	_, err := engine.Translate("Hello", "auto")
	if !errors.Is(err, errTranslateBad) && !errors.Is(err, errTranslateDown) {
		t.Fatalf("err = %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "翻译失败") {
		t.Fatalf("error should be Chinese, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func googleResponse(status int, body string, hits *atomic.Int32) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hits.Add(1)
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})
}

func withGoogleSplit(base http.RoundTripper, google http.RoundTripper) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Host, "googleapis") || strings.Contains(req.URL.Path, "translate_a") {
			return google.RoundTrip(req)
		}
		return base.RoundTrip(req)
	})
}

func TestGoogleUsesCoreProxyBeforeDirect(t *testing.T) {
	var directGoogle, coreGoogle atomic.Int32
	directSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bing down", http.StatusBadGateway)
	}))
	defer directSrv.Close()

	const googleBody = `[[["你好","Hello",null,null,1]],null,"en"]`
	direct := &http.Client{Transport: withGoogleSplit(directSrv.Client().Transport, googleResponse(http.StatusOK, `[["bad"]]`, &directGoogle)), Timeout: 2 * time.Second}
	core := &http.Client{Transport: googleResponse(http.StatusOK, googleBody, &coreGoogle), Timeout: 2 * time.Second}
	engine := newTranslateEngine(direct, nil)
	engine.core = core
	engine.bingPages = []string{directSrv.URL + "/translator"}
	engine.googleURL = "https://translate.googleapis.com/translate_a/single"

	result, err := engine.Translate("Hello", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "你好" || result.Provider != "google" {
		t.Fatalf("result = %+v", result)
	}
	if coreGoogle.Load() == 0 {
		t.Fatal("google was not sent through the core proxy")
	}
	if directGoogle.Load() != 0 {
		t.Fatal("direct client was used for google while the core proxy succeeded")
	}
}

func TestGoogleFallsBackToDirectWhenCoreProxyFails(t *testing.T) {
	var directGoogle, coreGoogle atomic.Int32
	directSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bing down", http.StatusBadGateway)
	}))
	defer directSrv.Close()

	const googleBody = `[[["你好","Hello",null,null,1]],null,"en"]`
	direct := &http.Client{Transport: withGoogleSplit(directSrv.Client().Transport, googleResponse(http.StatusOK, googleBody, &directGoogle)), Timeout: 2 * time.Second}
	core := &http.Client{Transport: googleResponse(http.StatusBadGateway, "down", &coreGoogle), Timeout: 2 * time.Second}
	engine := newTranslateEngine(direct, nil)
	engine.core = core
	engine.bingPages = []string{directSrv.URL + "/translator"}
	engine.googleURL = "https://translate.googleapis.com/translate_a/single"

	result, err := engine.Translate("Hello", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "google" || result.Text != "你好" {
		t.Fatalf("result = %+v", result)
	}
	if coreGoogle.Load() == 0 || directGoogle.Load() == 0 {
		t.Fatalf("core=%d direct=%d", coreGoogle.Load(), directGoogle.Load())
	}
}

func TestBingStaysDirectWhenCoreProxyIsSet(t *testing.T) {
	var coreHits atomic.Int32
	directSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/translator") {
			_, _ = io.WriteString(w, bingPageHTML)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"detectedLanguage":{"language":"en"},"translations":[{"text":"你好","to":"zh-Hans"}]}]`)
	}))
	defer directSrv.Close()

	engine := newTranslateEngine(directSrv.Client(), nil)
	engine.core = &http.Client{Transport: googleResponse(http.StatusBadGateway, "no", &coreHits), Timeout: 2 * time.Second}
	engine.bingPages = []string{directSrv.URL + "/translator"}
	engine.googleURL = "https://translate.googleapis.com/translate_a/single"

	result, err := engine.Translate("Hello", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "bing" || result.Text != "你好" {
		t.Fatalf("result = %+v", result)
	}
	if coreHits.Load() != 0 {
		t.Fatalf("bing used the core proxy, hits=%d", coreHits.Load())
	}
}
