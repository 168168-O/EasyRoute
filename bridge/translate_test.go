package bridge

import (
	"context"
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
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = server.URL + "/mymemory"

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

func TestTranslateUsesMyMemoryWhenCoreIsDown(t *testing.T) {
	var googleHits, memoryHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Host, "clients5") || strings.Contains(r.URL.Path, "translate_a") {
			googleHits.Add(1)
			t.Error("google was dialed while the core was down")
		}
		if strings.Contains(r.URL.Path, "/mymemory") {
			memoryHits.Add(1)
			if r.URL.Query().Get("langpair") != "zh-CN|en" {
				t.Errorf("langpair = %q", r.URL.Query().Get("langpair"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"responseStatus":200,"responseData":{"translatedText":"Hello"}}`)
			return
		}
		http.Error(w, "bing down", http.StatusBadGateway)
	}))
	defer server.Close()

	engine := newTranslateEngine(server.Client(), nil)
	engine.bingPages = []string{server.URL + "/translator"}
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = server.URL + "/mymemory"

	result, err := engine.Translate("你好", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Hello" || result.Provider != "mymemory" || result.TargetLang != "en" {
		t.Fatalf("result = %+v", result)
	}
	if memoryHits.Load() == 0 || googleHits.Load() != 0 {
		t.Fatalf("memory=%d google=%d", memoryHits.Load(), googleHits.Load())
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
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = server.URL + "/mymemory"

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
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = server.URL + "/mymemory"
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

func TestGoogleUsesCoreProxyAndSkipsDirect(t *testing.T) {
	var directGoogle, coreGoogle, memoryHits atomic.Int32
	directSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/mymemory") {
			memoryHits.Add(1)
		}
		http.Error(w, "not used", http.StatusBadGateway)
	}))
	defer directSrv.Close()

	const googleBody = `[[["你好","Hello",null,null,1]],null,"en"]`
	direct := &http.Client{Transport: withGoogleSplit(directSrv.Client().Transport, googleResponse(http.StatusOK, `[["bad"]]`, &directGoogle)), Timeout: 2 * time.Second}
	core := &http.Client{Transport: googleResponse(http.StatusOK, googleBody, &coreGoogle), Timeout: 2 * time.Second}
	engine := newTranslateEngine(direct, nil)
	engine.core = core
	engine.bingPages = []string{directSrv.URL + "/translator"}
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = directSrv.URL + "/mymemory"

	result, err := engine.Translate("DM me the price", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "你好" || result.Provider != "google" {
		t.Fatalf("result = %+v", result)
	}
	if coreGoogle.Load() == 0 {
		t.Fatal("google was not sent through the core proxy")
	}
	if directGoogle.Load() != 0 || memoryHits.Load() != 0 {
		t.Fatalf("direct google=%d mymemory=%d", directGoogle.Load(), memoryHits.Load())
	}
}

func TestGoogleProxyFailureUsesMyMemoryNotDirect(t *testing.T) {
	var directGoogle, coreGoogle, memoryHits atomic.Int32
	directSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/mymemory") {
			memoryHits.Add(1)
			if pair := r.URL.Query().Get("langpair"); pair != "en|zh-CN" {
				t.Errorf("langpair = %q", pair)
			}
			_, _ = io.WriteString(w, `{"responseStatus":200,"responseData":{"translatedText":"给我发价格"}}`)
			return
		}
		http.Error(w, "bing down", http.StatusBadGateway)
	}))
	defer directSrv.Close()

	direct := &http.Client{Transport: withGoogleSplit(directSrv.Client().Transport, googleResponse(http.StatusOK, `[[["direct should not run"]]]`, &directGoogle)), Timeout: 2 * time.Second}
	core := &http.Client{Transport: googleResponse(http.StatusBadGateway, "down", &coreGoogle), Timeout: 2 * time.Second}
	engine := newTranslateEngine(direct, nil)
	engine.core = core
	engine.bingPages = []string{directSrv.URL + "/translator"}
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = directSrv.URL + "/mymemory"

	result, err := engine.Translate("DM me the price", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "mymemory" || result.Text != "给我发价格" {
		t.Fatalf("result = %+v", result)
	}
	if coreGoogle.Load() == 0 || memoryHits.Load() == 0 {
		t.Fatalf("core=%d memory=%d", coreGoogle.Load(), memoryHits.Load())
	}
	if directGoogle.Load() != 0 {
		t.Fatal("google direct was attempted after the proxy failed")
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
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = directSrv.URL + "/mymemory"

	result, err := engine.Translate("Hello", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "bing" || result.Text != "你好" {
		t.Fatalf("result = %+v", result)
	}
	if coreHits.Load() == 0 {
		t.Fatal("google was not tried through the core before the direct fallback")
	}
}

func TestParseMyMemory(t *testing.T) {
	text, err := parseMyMemory([]byte(`{"responseStatus":200,"responseData":{"translatedText":"Tom&#39;s price"}}`))
	if err != nil || text != "Tom's price" {
		t.Fatalf("text=%q err=%v", text, err)
	}
	if _, err = parseMyMemory([]byte(`{"responseStatus":200,"responseData":{"translatedText":"MYMEMORY WARNING: YOU USED ALL AVAILABLE FREE TRANSLATIONS FOR TODAY"}}`)); err == nil {
		t.Fatal("warning was accepted")
	}
}

func TestMyMemoryLangPairForSlang(t *testing.T) {
	if got := myMemorySource("DM me the price", ""); got != "en" {
		t.Fatalf("source = %q", got)
	}
	if got := myMemoryLang("zh-Hans"); got != "zh-CN" {
		t.Fatalf("target = %q", got)
	}
}

func TestTranslateWorstCaseStaysUnderBudget(t *testing.T) {
	block := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	var directGoogle atomic.Int32
	direct := &http.Client{Transport: withGoogleSplit(block, googleResponse(http.StatusOK, `[[["should not run"]]]`, &directGoogle)), Timeout: 20 * time.Second}
	core := &http.Client{Transport: block, Timeout: 20 * time.Second}
	engine := newTranslateEngine(direct, nil)
	engine.core = core
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = "https://api.mymemory.translated.net/get"
	engine.bingPages = []string{"https://cn.bing.com/translator"}

	start := time.Now()
	_, err := engine.Translate("Hello", "auto")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected a failure when every engine hangs")
	}
	// Google (4s) and Bing (6s) run together, then MyMemory gets 4s.
	if elapsed > 12*time.Second {
		t.Fatalf("worst case took %s", elapsed)
	}
	if elapsed < 9*time.Second {
		t.Fatalf("chain returned in %s, before Bing and MyMemory timed out", elapsed)
	}
	if directGoogle.Load() != 0 {
		t.Fatal("a hanging google direct attempt was started")
	}
}

func TestGoogleAndBingRunTogetherAndSkipMyMemory(t *testing.T) {
	var googleStart, bingStart atomic.Int64
	var memoryHits atomic.Int32
	directSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/mymemory") {
			memoryHits.Add(1)
			http.Error(w, "should not run", http.StatusBadGateway)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/translator") {
			if bingStart.CompareAndSwap(0, time.Now().UnixNano()) {
				time.Sleep(80 * time.Millisecond)
			}
			_, _ = io.WriteString(w, bingPageHTML)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"detectedLanguage":{"language":"en"},"translations":[{"text":"哈哈","to":"zh-Hans"}]}]`)
	}))
	defer directSrv.Close()

	core := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		googleStart.Store(time.Now().UnixNano())
		time.Sleep(80 * time.Millisecond)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`[[["大笑","lol",null,null,1]],null,"en"]`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}), Timeout: 2 * time.Second}

	engine := newTranslateEngine(directSrv.Client(), nil)
	engine.core = core
	engine.bingPages = []string{directSrv.URL + "/translator"}
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = directSrv.URL + "/mymemory"

	bundle, err := engine.TranslateAll("lol", "auto", aiSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Results) != 2 || bundle.Results[0].Provider != "google" || bundle.Results[1].Provider != "bing" {
		t.Fatalf("bundle = %+v", bundle.Results)
	}
	if bundle.Results[0].Text != "大笑" || bundle.Results[1].Text != "哈哈" {
		t.Fatalf("texts = %+v", bundle.Results)
	}
	if memoryHits.Load() != 0 {
		t.Fatal("MyMemory ran even though Google and Bing answered")
	}
	if googleStart.Load() == 0 || bingStart.Load() == 0 {
		t.Fatal("one of the engines never started")
	}
	gap := googleStart.Load() - bingStart.Load()
	if gap < 0 {
		gap = -gap
	}
	if gap > int64(500*time.Millisecond) {
		t.Fatalf("engines did not overlap, gap %s", time.Duration(gap))
	}
}

func TestAIResultComesFirstAndDomesticStaysDirect(t *testing.T) {
	var directHits, proxyHits atomic.Int32
	direct := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		directHits.Add(1)
		if req.URL.Host != "api.deepseek.com" {
			return nil, errors.New("not the model")
		}
		if req.Header.Get("Authorization") != "Bearer sk-domestic" {
			t.Errorf("auth header changed")
		}
		if strings.Contains(req.Header.Get("Authorization"), "sk-domestic") && strings.Contains(req.URL.String(), "sk-domestic") {
			t.Error("key was placed in the URL")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"哈哈"}}]}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}), Timeout: 2 * time.Second}
	proxy := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		proxyHits.Add(1)
		if strings.Contains(req.URL.Host, "deepseek") || strings.Contains(req.URL.Host, "aliyuncs") || strings.Contains(req.URL.Host, "moonshot") || strings.Contains(req.URL.Host, "bigmodel") {
			t.Errorf("domestic model used the proxy: %s", req.URL.Host)
		}
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("google skipped")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}), Timeout: 2 * time.Second}

	engine := newTranslateEngine(direct, proxy)
	engine.core = nil
	engine.bingPages = []string{"http://127.0.0.1:1/translator"}
	engine.googleURL = "https://clients5.google.com/translate_a/single"
	engine.myMemoryURL = "http://127.0.0.1:1/mymemory"
	engine.direct = direct

	bundle, err := engine.TranslateAll("lol", "auto", aiSettings{
		Provider: "openai",
		BaseURL:  "https://api.deepseek.com/v1",
		APIKey:   "sk-domestic",
		Model:    "deepseek-chat",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Results) == 0 || bundle.Results[0].Provider != "openai" || bundle.Results[0].Text != "哈哈" || bundle.Results[0].Label != "deepseek-chat" {
		t.Fatalf("bundle = %+v", bundle)
	}
	if directHits.Load() == 0 {
		t.Fatal("domestic model was not dialed directly")
	}
}

func TestForeignAIUsesProxyAndHidesTheKey(t *testing.T) {
	var directHits atomic.Int32
	direct := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		directHits.Add(1)
		return nil, errors.New("direct dial")
	}), Timeout: 2 * time.Second}
	core := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Host, "api.openai.com") {
			t.Errorf("host = %s", req.URL.Host)
		}
		if strings.Contains(req.URL.String(), "sk-foreign") {
			t.Error("key leaked into the URL")
		}
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad key sk-foreign"}}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}), Timeout: 2 * time.Second}
	engine := newTranslateEngine(direct, nil)
	engine.core = core
	_, err := engine.translateAI(context.Background(), aiSettings{
		Provider: "openai",
		BaseURL:  "https://api.openai.com/v1",
		APIKey:   "sk-foreign",
		Model:    "gpt-4o-mini",
	}, "lol", "", "zh-Hans")
	if err == nil || strings.Contains(err.Error(), "sk-foreign") {
		t.Fatalf("err = %v", err)
	}
	if directHits.Load() != 0 {
		t.Fatal("foreign model was dialed directly")
	}
}

func TestForeignAIWithoutProxyDoesNotDial(t *testing.T) {
	var hits atomic.Int32
	direct := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hits.Add(1)
		return nil, errors.New("dialed")
	})}
	engine := newTranslateEngine(direct, nil)
	_, err := engine.translateAI(context.Background(), aiSettings{
		Provider: "deepl",
		DeepLKey: "free-key:fx",
	}, "lol", "", "zh-Hans")
	if err == nil || !strings.Contains(err.Error(), "代理") {
		t.Fatalf("err = %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("DeepL was dialed without a proxy")
	}
	if !strings.Contains(deeplEndpoint("free-key:fx"), "api-free.deepl.com") {
		t.Fatal("free key did not use the free endpoint")
	}
	if strings.Contains(deeplEndpoint("pro-key"), "api-free") {
		t.Fatal("pro key used the free endpoint")
	}
}

func TestBingPageTokenTTL(t *testing.T) {
	html := `<script>var params_AbusePreventionHelper = [1000,"tok",3600000];</script><script>IG:"AABBCCDDEEFF00112233445566778899"</script><div data-iid="translator.5023"></div>`
	creds, err := parseBingPage(html, "https://www.bing.com/translator")
	if err != nil {
		t.Fatal(err)
	}
	want := time.UnixMilli(1000).Add(3600*time.Second - 30*time.Second)
	if !creds.until.Equal(want) {
		t.Fatalf("until = %s want %s", creds.until, want)
	}
}

func TestDirectTranslateClientKeepsCookies(t *testing.T) {
	if directTranslateClient().Jar == nil {
		t.Fatal("translation client has no cookie jar")
	}
}

func TestDomesticAIHosts(t *testing.T) {
	for _, host := range []string{"api.deepseek.com", "dashscope.aliyuncs.com", "open.bigmodel.cn", "api.moonshot.cn"} {
		if !isDomesticAIHost(host) {
			t.Fatalf("%s should stay direct", host)
		}
	}
	for _, host := range []string{"api.openai.com", "api.deepl.com", "api-free.deepl.com"} {
		if isDomesticAIHost(host) {
			t.Fatalf("%s should use the proxy", host)
		}
	}
}
