package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultGoogleURL   = "https://translate.googleapis.com/translate_a/single"
	translateUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0"
)

var (
	bingKeyPattern = regexp.MustCompile(`params_AbusePreventionHelper\s*=\s*\[(\d+),"([^"]+)"`)
	bingIGPattern  = regexp.MustCompile(`IG:"([A-F0-9]+)"`)
	bingIIDPattern = regexp.MustCompile(`data-iid="(translator\.[^"]+)"`)
)

// The old edge.microsoft.com/translate/auth endpoint now returns 404.
// Bing's public translator page still issues a short-lived token that works from China.
var defaultBingPages = []string{
	"https://cn.bing.com/translator",
	"https://www.bing.com/translator?mkt=zh-CN",
}

var (
	errTranslateEmpty   = errors.New("请先输入要翻译的内容")
	errTranslateTooLong = errors.New("内容太长了，请分成几段再翻译")
	errTranslateDown    = errors.New("翻译失败：连不上翻译服务。请检查网络后再试。")
	errTranslateBad     = errors.New("翻译失败：翻译服务没有返回可用的结果。请稍后再试。")
)

// TranslateResult is what the 翻译 page shows.
type TranslateResult struct {
	Text       string `json:"text"`
	SourceLang string `json:"sourceLang"`
	TargetLang string `json:"targetLang"`
	Provider   string `json:"provider"`
}

// isMostlyChinese reports whether the message should be treated as Chinese.
// English and every other language go the other way.
func isMostlyChinese(text string) bool {
	han, letters := 0, 0
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			han++
			letters++
		case unicode.IsLetter(r):
			letters++
		}
	}
	return han > 0 && han*2 >= letters
}

// ResolveLanguages picks a Bing source and target.
// An empty or "auto" target means: Chinese → English, everything else → Chinese.
// An explicit target is kept; the source is left blank so the service can detect it,
// unless the text is already in that language, in which case the other side is used.
func ResolveLanguages(text, target string) (from, to string) {
	target = strings.TrimSpace(target)
	if target == "" || strings.EqualFold(target, "auto") {
		if isMostlyChinese(text) {
			return "zh-Hans", "en"
		}
		return "", "zh-Hans"
	}
	to = normalizeLang(target)
	if isMostlyChinese(text) && isChineseCode(to) {
		return "en", to
	}
	if !isMostlyChinese(text) && to == "en" {
		return "zh-Hans", "en"
	}
	return "", to
}

func isChineseCode(code string) bool {
	switch strings.ToLower(code) {
	case "zh", "zh-hans", "zh-cn", "zh-hant", "zh-tw":
		return true
	default:
		return false
	}
}

func normalizeLang(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "zh", "zh-cn", "zh-hans", "chinese", "中文":
		return "zh-Hans"
	case "zh-tw", "zh-hant":
		return "zh-Hant"
	case "en", "english":
		return "en"
	case "ja", "jp", "japanese":
		return "ja"
	case "ko", "kr", "korean":
		return "ko"
	default:
		return strings.TrimSpace(code)
	}
}

func googleLang(code string) string {
	switch code {
	case "zh-Hans", "":
		return "zh-CN"
	case "zh-Hant":
		return "zh-TW"
	default:
		return code
	}
}

func parseBingTranslations(body []byte) (text, detected, to string, err error) {
	var payload []struct {
		DetectedLanguage struct {
			Language string `json:"language"`
		} `json:"detectedLanguage"`
		Translations []struct {
			Text string `json:"text"`
			To   string `json:"to"`
		} `json:"translations"`
	}
	if err = json.Unmarshal(body, &payload); err != nil {
		return "", "", "", err
	}
	if len(payload) == 0 || len(payload[0].Translations) == 0 {
		return "", "", "", errors.New("empty bing payload")
	}
	var b strings.Builder
	for _, item := range payload {
		for _, part := range item.Translations {
			b.WriteString(part.Text)
			if to == "" {
				to = part.To
			}
		}
		if detected == "" {
			detected = item.DetectedLanguage.Language
		}
	}
	if b.Len() == 0 {
		return "", "", "", errors.New("empty bing text")
	}
	return b.String(), detected, to, nil
}

func parseGoogleTranslations(body []byte) (string, error) {
	var data []any
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("empty google payload")
	}
	sentences, ok := data[0].([]any)
	if !ok {
		return "", errors.New("unexpected google payload")
	}
	var b strings.Builder
	for _, item := range sentences {
		row, ok := item.([]any)
		if !ok || len(row) == 0 {
			continue
		}
		text, ok := row[0].(string)
		if !ok {
			continue
		}
		b.WriteString(text)
	}
	if b.Len() == 0 {
		return "", errors.New("empty google text")
	}
	return b.String(), nil
}

type bingCreds struct {
	key     string
	token   string
	ig      string
	iid     string
	postURL string
	until   time.Time
}

type translateEngine struct {
	mu        sync.Mutex
	direct    *http.Client
	proxy     *http.Client
	bingPages []string
	googleURL string
	creds     bingCreds
}

func newTranslateEngine(direct, proxy *http.Client) *translateEngine {
	return &translateEngine{
		direct:    direct,
		proxy:     proxy,
		bingPages: append([]string(nil), defaultBingPages...),
		googleURL: defaultGoogleURL,
	}
}

func directTranslateClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Ignore HTTP_PROXY / the system proxy. Translation must leave the machine directly.
	transport.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	return &http.Client{Transport: transport, Timeout: 12 * time.Second}
}

func proxyTranslateClient(proxyURL string) *http.Client {
	proxyURL = strings.TrimSpace(proxyURL)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil || parsed.Host == "" {
			return nil
		}
		transport.Proxy = http.ProxyURL(parsed)
	} else if !envProxyConfigured() {
		return nil
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}
	return &http.Client{Transport: transport, Timeout: 12 * time.Second}
}

func envProxyConfigured() bool {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func (e *translateEngine) do(method, rawURL, body string, headers map[string]string) (int, []byte, error) {
	status, payload, _, err := e.doClient(e.direct, method, rawURL, body, headers)
	if e.proxy == nil || (err == nil && status < 500) {
		return status, payload, err
	}
	status, payload, _, err = e.doClient(e.proxy, method, rawURL, body, headers)
	return status, payload, err
}

func (e *translateEngine) doClient(client *http.Client, method, rawURL, body string, headers map[string]string) (int, []byte, string, error) {
	if client == nil {
		return 0, nil, "", errors.New("no client")
	}
	req, err := http.NewRequest(method, rawURL, bytes.NewReader([]byte(body)))
	if err != nil {
		return 0, nil, "", err
	}
	req.Header.Set("User-Agent", translateUserAgent)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	final := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	if err != nil {
		return resp.StatusCode, nil, final, err
	}
	return resp.StatusCode, payload, final, nil
}

func parseBingPage(html, pageURL string) (bingCreds, error) {
	key := bingKeyPattern.FindStringSubmatch(html)
	ig := bingIGPattern.FindStringSubmatch(html)
	iid := bingIIDPattern.FindStringSubmatch(html)
	if key == nil || ig == nil || iid == nil {
		return bingCreds{}, errTranslateBad
	}
	parsed, err := url.Parse(pageURL)
	if err != nil || parsed.Host == "" {
		return bingCreds{}, errTranslateBad
	}
	expiryMS, err := strconvParseInt(key[1])
	if err != nil {
		return bingCreds{}, errTranslateBad
	}
	post := parsed.Scheme + "://" + parsed.Host + "/ttranslatev3?isVertical=1&IG=" + ig[1] + "&IID=" + url.QueryEscape(iid[1])
	return bingCreds{
		key:     key[1],
		token:   key[2],
		ig:      ig[1],
		iid:     iid[1],
		postURL: post,
		until:   time.UnixMilli(expiryMS).Add(-30 * time.Second),
	}, nil
}

func strconvParseInt(raw string) (int64, error) {
	var n int64
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, errTranslateBad
		}
		n = n*10 + int64(r-'0')
	}
	return n, nil
}

func (e *translateEngine) loadBingCreds(client *http.Client) (bingCreds, error) {
	e.mu.Lock()
	if e.creds.token != "" && time.Now().Before(e.creds.until) {
		creds := e.creds
		e.mu.Unlock()
		return creds, nil
	}
	pages := append([]string(nil), e.bingPages...)
	e.mu.Unlock()

	var last error = errTranslateDown
	for _, page := range pages {
		status, body, finalURL, err := e.doClient(client, http.MethodGet, page, "", map[string]string{
			"Accept": "text/html",
		})
		if err != nil || status != http.StatusOK {
			last = errTranslateDown
			continue
		}
		creds, err := parseBingPage(string(body), finalURL)
		if err != nil {
			last = err
			continue
		}
		e.mu.Lock()
		e.creds = creds
		e.mu.Unlock()
		return creds, nil
	}
	return bingCreds{}, last
}

func (e *translateEngine) bingOnce(client *http.Client, text, from, to string) (TranslateResult, error) {
	if client == nil {
		return TranslateResult{}, errTranslateDown
	}
	creds, err := e.loadBingCreds(client)
	if err != nil {
		return TranslateResult{}, err
	}
	source := from
	if source == "" {
		source = "auto-detect"
	}
	form := url.Values{}
	form.Set("fromLang", source)
	form.Set("to", to)
	form.Set("text", text)
	form.Set("token", creds.token)
	form.Set("key", creds.key)
	status, body, _, err := e.doClient(client, http.MethodPost, creds.postURL, form.Encode(), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Referer":      "https://www.bing.com/translator",
	})
	if err != nil {
		e.mu.Lock()
		e.creds = bingCreds{}
		e.mu.Unlock()
		return TranslateResult{}, errTranslateDown
	}
	if status != http.StatusOK {
		e.mu.Lock()
		e.creds = bingCreds{}
		e.mu.Unlock()
		return TranslateResult{}, errTranslateBad
	}
	translated, detected, gotTo, err := parseBingTranslations(body)
	if err != nil {
		return TranslateResult{}, errTranslateBad
	}
	if detected == "" {
		detected = from
	}
	if gotTo == "" {
		gotTo = to
	}
	return TranslateResult{
		Text:       translated,
		SourceLang: detected,
		TargetLang: gotTo,
		Provider:   "bing",
	}, nil
}

func (e *translateEngine) translateBing(text, from, to string) (TranslateResult, error) {
	result, err := e.bingOnce(e.direct, text, from, to)
	if err == nil {
		return result, nil
	}
	if e.proxy == nil || !errors.Is(err, errTranslateDown) {
		return TranslateResult{}, err
	}
	e.mu.Lock()
	e.creds = bingCreds{}
	e.mu.Unlock()
	return e.bingOnce(e.proxy, text, from, to)
}

func (e *translateEngine) translateGoogle(text, from, to string) (TranslateResult, error) {
	form := url.Values{}
	form.Set("q", text)
	source := from
	if source == "" {
		source = "auto"
	}
	endpoint := e.googleURL + "?client=gtx&sl=" + url.QueryEscape(googleLang(source)) + "&tl=" + url.QueryEscape(googleLang(to)) + "&dt=t"
	status, body, err := e.do(http.MethodPost, endpoint, form.Encode(), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if err != nil {
		return TranslateResult{}, errTranslateDown
	}
	if status != http.StatusOK {
		return TranslateResult{}, errTranslateBad
	}
	translated, err := parseGoogleTranslations(body)
	if err != nil {
		return TranslateResult{}, errTranslateBad
	}
	if source == "auto" {
		source = ""
	}
	return TranslateResult{
		Text:       translated,
		SourceLang: source,
		TargetLang: to,
		Provider:   "google",
	}, nil
}

func (e *translateEngine) Translate(text, target string) (TranslateResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return TranslateResult{}, errTranslateEmpty
	}
	if len([]rune(text)) > 5000 {
		return TranslateResult{}, errTranslateTooLong
	}
	from, to := ResolveLanguages(text, target)
	result, bingErr := e.translateBing(text, from, to)
	if bingErr == nil && strings.TrimSpace(result.Text) != "" {
		return result, nil
	}
	result, googleErr := e.translateGoogle(text, from, to)
	if googleErr == nil && strings.TrimSpace(result.Text) != "" {
		return result, nil
	}
	if bingErr != nil && errors.Is(bingErr, errTranslateDown) && (googleErr == nil || errors.Is(googleErr, errTranslateDown)) {
		return TranslateResult{}, errTranslateDown
	}
	return TranslateResult{}, errTranslateBad
}

var sharedTranslate = newTranslateEngine(nil, nil)

// Translate asks Bing's public translator directly, then Google if Bing fails.
// The first attempt never uses a proxy. A proxy is used only when the direct attempt fails.
func (a *App) Translate(text string, target string) FlagResult {
	proxyURL := ""
	if got := a.GetSystemProxy(); got.Flag {
		proxyURL = got.Data
	}
	sharedTranslate.mu.Lock()
	if sharedTranslate.direct == nil {
		sharedTranslate.direct = directTranslateClient()
	}
	sharedTranslate.proxy = proxyTranslateClient(proxyURL)
	sharedTranslate.mu.Unlock()
	result, err := sharedTranslate.Translate(text, target)
	if err != nil {
		return FlagResult{false, err.Error()}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return FlagResult{false, errTranslateBad.Error()}
	}
	return FlagResult{true, string(raw)}
}
