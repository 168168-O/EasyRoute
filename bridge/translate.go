package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// clients5 is reachable through the core. A direct dial from mainland telecom hangs.
	defaultGoogleURL   = "https://clients5.google.com/translate_a/single"
	defaultMyMemoryURL = "https://api.mymemory.translated.net/get"
	translateUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0"
	// Google through the core (4s) and Bing direct (6s) run together.
	// MyMemory (4s) runs only after both fail. An AI key gets 15s in parallel.
	googleProxyTimeout = 4 * time.Second
	bingTimeout        = 6 * time.Second
	myMemoryTimeout    = 4 * time.Second
	aiTimeout          = 15 * time.Second
)

var (
	// The third number is a TTL in milliseconds. The first number is the issue time, not the expiry.
	bingKeyPattern = regexp.MustCompile(`params_AbusePreventionHelper\s*=\s*\[(\d+),"([^"]+)"(?:,(\d+))?`)
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
	Label      string `json:"label,omitempty"`
}

// TranslateBundle is the comparison the 翻译 page shows.
// Results are ordered with the AI translation first when a key is set.
type TranslateBundle struct {
	Results []TranslateResult `json:"results"`
	Note    string            `json:"note,omitempty"`
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
	mu          sync.Mutex
	direct      *http.Client
	proxy       *http.Client
	core        *http.Client
	bingPages   []string
	googleURL   string
	myMemoryURL string
	creds       bingCreds
	credsClient *http.Client
}

func newTranslateEngine(direct, proxy *http.Client) *translateEngine {
	return &translateEngine{
		direct:      direct,
		proxy:       proxy,
		bingPages:   append([]string(nil), defaultBingPages...),
		googleURL:   defaultGoogleURL,
		myMemoryURL: defaultMyMemoryURL,
	}
}

func directTranslateClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Ignore HTTP_PROXY / the system proxy. Translation must leave the machine directly.
	transport.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	return &http.Client{Transport: transport, Jar: newTranslateJar(), Timeout: 12 * time.Second}
}

func newTranslateJar() http.CookieJar {
	jar, _ := cookiejar.New(nil)
	return jar
}

func explicitProxyClient(proxyURL string) *http.Client {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(parsed)
	return &http.Client{Transport: transport, Jar: newTranslateJar(), Timeout: 12 * time.Second}
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
	return &http.Client{Transport: transport, Jar: newTranslateJar(), Timeout: 12 * time.Second}
}

func envProxyConfigured() bool {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func (e *translateEngine) do(ctx context.Context, method, rawURL, body string, headers map[string]string) (int, []byte, error) {
	status, payload, _, err := e.doClient(ctx, e.direct, method, rawURL, body, headers)
	if e.proxy == nil || (err == nil && status < 500) {
		return status, payload, err
	}
	status, payload, _, err = e.doClient(ctx, e.proxy, method, rawURL, body, headers)
	return status, payload, err
}

func (e *translateEngine) doClient(ctx context.Context, client *http.Client, method, rawURL, body string, headers map[string]string) (int, []byte, string, error) {
	if client == nil {
		return 0, nil, "", errors.New("no client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader([]byte(body)))
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
	issuedMS, err := strconvParseInt(key[1])
	if err != nil {
		return bingCreds{}, errTranslateBad
	}
	until := time.UnixMilli(issuedMS).Add(-30 * time.Second)
	if key[3] != "" {
		ttlMS, err := strconvParseInt(key[3])
		if err != nil {
			return bingCreds{}, errTranslateBad
		}
		until = time.UnixMilli(issuedMS).Add(time.Duration(ttlMS) * time.Millisecond).Add(-30 * time.Second)
	}
	post := parsed.Scheme + "://" + parsed.Host + "/ttranslatev3?isVertical=1&IG=" + ig[1] + "&IID=" + url.QueryEscape(iid[1])
	return bingCreds{
		key:     key[1],
		token:   key[2],
		ig:      ig[1],
		iid:     iid[1],
		postURL: post,
		until:   until,
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

func (e *translateEngine) loadBingCreds(ctx context.Context, client *http.Client) (bingCreds, error) {
	e.mu.Lock()
	if e.creds.token != "" && time.Now().Before(e.creds.until) && e.credsClient == client {
		creds := e.creds
		e.mu.Unlock()
		return creds, nil
	}
	pages := append([]string(nil), e.bingPages...)
	e.mu.Unlock()

	var last error = errTranslateDown
	for _, page := range pages {
		status, body, finalURL, err := e.doClient(ctx, client, http.MethodGet, page, "", map[string]string{
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
		e.credsClient = client
		e.mu.Unlock()
		return creds, nil
	}
	return bingCreds{}, last
}

func (e *translateEngine) bingOnce(ctx context.Context, client *http.Client, text, from, to string) (TranslateResult, error) {
	if client == nil {
		return TranslateResult{}, errTranslateDown
	}
	creds, err := e.loadBingCreds(ctx, client)
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
	status, body, _, err := e.doClient(ctx, client, http.MethodPost, creds.postURL, form.Encode(), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Referer":      "https://www.bing.com/translator",
	})
	if err != nil {
		e.mu.Lock()
		e.creds = bingCreds{}
		e.credsClient = nil
		e.mu.Unlock()
		return TranslateResult{}, errTranslateDown
	}
	if status != http.StatusOK {
		e.mu.Lock()
		e.creds = bingCreds{}
		e.credsClient = nil
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

func (e *translateEngine) translateBing(ctx context.Context, text, from, to string) (TranslateResult, error) {
	result, err := e.bingOnce(ctx, e.direct, text, from, to)
	if err == nil {
		return result, nil
	}
	if e.proxy == nil || !errors.Is(err, errTranslateDown) || ctx.Err() != nil {
		return TranslateResult{}, err
	}
	e.mu.Lock()
	e.creds = bingCreds{}
	e.credsClient = nil
	e.mu.Unlock()
	return e.bingOnce(ctx, e.proxy, text, from, to)
}

func myMemoryLang(code string) string {
	switch code {
	case "zh-Hans", "":
		return "zh-CN"
	case "zh-Hant":
		return "zh-TW"
	default:
		return code
	}
}

// myMemorySource picks an explicit pair. MyMemory has no auto-detect.
func myMemorySource(text, from string) string {
	if from != "" {
		return myMemoryLang(from)
	}
	var hangul, kana, cyrillic, han, letters int
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Hangul, r):
			hangul++
			letters++
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			kana++
			letters++
		case unicode.Is(unicode.Cyrillic, r):
			cyrillic++
			letters++
		case unicode.Is(unicode.Han, r):
			han++
			letters++
		case unicode.IsLetter(r):
			letters++
		}
	}
	switch {
	case kana > 0 && kana >= hangul && kana >= cyrillic:
		return "ja"
	case hangul > 0 && hangul >= cyrillic:
		return "ko"
	case cyrillic > 0:
		return "ru"
	case han > 0:
		return "zh-CN"
	default:
		return "en"
	}
}

func parseMyMemory(body []byte) (string, error) {
	var payload struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		ResponseStatus  int    `json:"responseStatus"`
		ResponseDetails string `json:"responseDetails"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	text := strings.TrimSpace(html.UnescapeString(payload.ResponseData.TranslatedText))
	details := strings.ToUpper(payload.ResponseDetails + " " + text)
	if payload.ResponseStatus != http.StatusOK || text == "" || strings.Contains(details, "MYMEMORY WARNING") || strings.Contains(details, "INVALID LANGUAGE PAIR") {
		return "", errTranslateBad
	}
	return text, nil
}

func (e *translateEngine) translateGoogle(ctx context.Context, client *http.Client, text, from, to string) (TranslateResult, error) {
	if client == nil {
		return TranslateResult{}, errTranslateDown
	}
	source := from
	if source == "" {
		source = "auto"
	}
	// Direct clients5 hangs on mainland telecom. This client is only the core inbound.
	endpoint := e.googleURL + "?client=gtx&sl=" + url.QueryEscape(googleLang(source)) + "&tl=" + url.QueryEscape(googleLang(to)) + "&dt=t&q=" + url.QueryEscape(text)
	status, body, _, err := e.doClient(ctx, client, http.MethodGet, endpoint, "", nil)
	if err != nil || status >= 500 {
		return TranslateResult{}, errTranslateDown
	}
	if status != http.StatusOK {
		return TranslateResult{}, errTranslateBad
	}
	translated, err := parseGoogleTranslations(body)
	if err != nil {
		return TranslateResult{}, errTranslateBad
	}
	detected := source
	if detected == "auto" {
		detected = ""
	}
	return TranslateResult{
		Text:       translated,
		SourceLang: detected,
		TargetLang: to,
		Provider:   "google",
	}, nil
}

func (e *translateEngine) translateMyMemory(ctx context.Context, text, from, to string) (TranslateResult, error) {
	source := myMemorySource(text, from)
	target := myMemoryLang(to)
	if source == target {
		if target == "zh-CN" {
			source = "en"
		} else {
			target = "zh-CN"
		}
	}
	endpoint := e.myMemoryURL + "?q=" + url.QueryEscape(text) + "&langpair=" + url.QueryEscape(source+"|"+target)
	status, body, _, err := e.doClient(ctx, e.direct, http.MethodGet, endpoint, "", nil)
	if err != nil || status >= 500 {
		return TranslateResult{}, errTranslateDown
	}
	translated, err := parseMyMemory(body)
	if err != nil {
		return TranslateResult{}, err
	}
	detected := from
	if detected == "" {
		detected = source
	}
	return TranslateResult{
		Text:       translated,
		SourceLang: detected,
		TargetLang: to,
		Provider:   "mymemory",
	}, nil
}

func usable(result TranslateResult, err error) bool {
	return err == nil && strings.TrimSpace(result.Text) != ""
}

func (e *translateEngine) Translate(text, target string) (TranslateResult, error) {
	bundle, err := e.TranslateAll(text, target, aiSettings{})
	if err != nil {
		return TranslateResult{}, err
	}
	if len(bundle.Results) == 0 {
		return TranslateResult{}, errTranslateBad
	}
	return bundle.Results[0], nil
}

// TranslateAll runs Google (core proxy only) and Bing (direct) together.
// MyMemory runs only when both of those return nothing. An AI key, when set, is
// requested at the same time and placed first.
func (e *translateEngine) TranslateAll(text, target string, ai aiSettings) (TranslateBundle, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return TranslateBundle{}, errTranslateEmpty
	}
	if len([]rune(text)) > 5000 {
		return TranslateBundle{}, errTranslateTooLong
	}
	from, to := ResolveLanguages(text, target)
	aiOn := ai.enabled()

	var googleRes, bingRes, aiRes TranslateResult
	var googleErr, bingErr, aiErr error
	var group sync.WaitGroup

	if aiOn {
		group.Add(1)
		go func() {
			defer group.Done()
			ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
			defer cancel()
			aiRes, aiErr = e.translateAI(ctx, ai, text, from, to)
		}()
	}
	if e.core != nil {
		group.Add(1)
		go func() {
			defer group.Done()
			ctx, cancel := context.WithTimeout(context.Background(), googleProxyTimeout)
			defer cancel()
			googleRes, googleErr = e.translateGoogle(ctx, e.core, text, from, to)
		}()
	}
	group.Add(1)
	go func() {
		defer group.Done()
		ctx, cancel := context.WithTimeout(context.Background(), bingTimeout)
		defer cancel()
		bingRes, bingErr = e.translateBing(ctx, text, from, to)
	}()
	group.Wait()

	results := make([]TranslateResult, 0, 3)
	if aiOn && usable(aiRes, aiErr) {
		results = append(results, aiRes)
	}
	if usable(googleRes, googleErr) {
		results = append(results, googleRes)
	}
	if usable(bingRes, bingErr) {
		results = append(results, bingRes)
	}
	note := ""
	if aiOn && !usable(aiRes, aiErr) && len(results) > 0 {
		note = "AI 这次没有返回，下面是对照译文。"
	}
	if len(results) > 0 {
		if e.core == nil && usable(bingRes, bingErr) && !usable(googleRes, googleErr) && note == "" {
			note = "谷歌要等核心开着才走当前节点。这次用必应直连。"
		}
		return TranslateBundle{Results: results, Note: note}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), myMemoryTimeout)
	memory, err := e.translateMyMemory(ctx, text, from, to)
	cancel()
	if usable(memory, err) {
		return TranslateBundle{
			Results: []TranslateResult{memory},
			Note:    "谷歌和必应这次没返回，用了 MyMemory。",
		}, nil
	}
	if aiOn && aiErr != nil {
		return TranslateBundle{}, aiErr
	}
	if err != nil && errors.Is(err, errTranslateDown) {
		return TranslateBundle{}, errTranslateDown
	}
	if bingErr != nil && errors.Is(bingErr, errTranslateDown) {
		return TranslateBundle{}, errTranslateDown
	}
	return TranslateBundle{}, errTranslateBad
}

var sharedTranslate = newTranslateEngine(nil, nil)

func (a *App) prepareTranslate(coreProxy string) {
	proxyURL := ""
	if got := a.GetSystemProxy(); got.Flag {
		proxyURL = got.Data
	}
	sharedTranslate.mu.Lock()
	if sharedTranslate.direct == nil {
		sharedTranslate.direct = directTranslateClient()
	}
	sharedTranslate.proxy = proxyTranslateClient(proxyURL)
	sharedTranslate.core = explicitProxyClient(coreProxy)
	sharedTranslate.mu.Unlock()
}

// Translate runs Google through the core and Bing directly, at the same time.
// MyMemory is used only when both return nothing. settingsJSON is the local AI
// key, kept on this machine. Google is never dialed directly.
func (a *App) Translate(text string, target string, coreProxy string, settingsJSON string) FlagResult {
	a.prepareTranslate(coreProxy)
	ai, err := parseAISettings(settingsJSON)
	if err != nil {
		return FlagResult{Flag: false, Data: "AI 设置读不出来"}
	}
	bundle, err := sharedTranslate.TranslateAll(text, target, ai)
	if err != nil {
		return FlagResult{Flag: false, Data: err.Error()}
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		return FlagResult{Flag: false, Data: errTranslateBad.Error()}
	}
	return FlagResult{Flag: true, Data: string(raw)}
}

// TestTranslateAI checks a pasted key with one short sentence. The key is not stored here.
func (a *App) TestTranslateAI(settingsJSON string, coreProxy string) FlagResult {
	ai, err := parseAISettings(settingsJSON)
	if err != nil || !ai.enabled() {
		return FlagResult{Flag: false, Data: "请先填写 API Key"}
	}
	a.prepareTranslate(coreProxy)
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
	defer cancel()
	result, err := sharedTranslate.translateAI(ctx, ai, "lol", "", "zh-Hans")
	if err != nil {
		return FlagResult{Flag: false, Data: err.Error()}
	}
	return FlagResult{Flag: true, Data: "测试通过：" + result.Text}
}
