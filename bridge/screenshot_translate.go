package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const screenshotDeepSeekPrompt = "你是翻译助手。把用户给出的文字翻译成简体中文。只输出译文，不要解释。"

var (
	deepseekEndpoint   = "https://api.deepseek.com/chat/completions"
	googleShotEndpoint = defaultGoogleURL
	myMemoryEndpoint   = "https://api.mymemory.translated.net/get"
	errNoShotText      = errors.New("没有识别到文字。请框选包含英文的区域。")
	errShotOCR         = errors.New("识别失败。请换一块更清晰的英文区域再试。")
	errShotOCRLang     = errors.New("识别失败。请确认 Windows 已安装光学字符识别语言。")
)

var (
	shotDeepSeekFn = callDeepSeek
	shotGoogleFn   = callGoogleProxy
	shotMemoryFn   = callMyMemory
)

func shotDirectClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
	return &http.Client{Transport: transport, Timeout: timeout}
}

// translateScreenshotText prefers a configured DeepSeek key.
// With no key, Google is used only through the running core proxy.
// With neither a key nor a proxy, MyMemory translates directly.
func translateScreenshotText(text, deepseekKey, coreProxy string) (string, string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", errNoShotText
	}
	if utf8.RuneCountInString(text) > 4000 {
		text = string([]rune(text)[:4000])
	}
	key := strings.TrimSpace(deepseekKey)
	proxy := strings.TrimSpace(coreProxy)
	if key != "" {
		translated, err := shotDeepSeekFn(shotDirectClient(25*time.Second), key, text)
		if err == nil && strings.TrimSpace(translated) != "" {
			return strings.TrimSpace(translated), "deepseek", nil
		}
	}
	if proxy != "" {
		translated, err := shotGoogleFn(proxy, text)
		if err != nil {
			return "", "", err
		}
		return translated, "google", nil
	}
	translated, err := shotMemoryFn(shotDirectClient(12*time.Second), text)
	if err != nil {
		return "", "", err
	}
	return translated, "mymemory", nil
}

func callDeepSeek(client *http.Client, key, text string) (string, error) {
	if client == nil {
		return "", errTranslateDown
	}
	body, err := json.Marshal(map[string]any{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": screenshotDeepSeekPrompt},
			{"role": "user", "content": text},
		},
		"temperature": 0.2,
	})
	if err != nil {
		return "", errTranslateBad
	}
	req, err := http.NewRequest(http.MethodPost, deepseekEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", errTranslateDown
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", translateUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", errTranslateDown
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", errTranslateDown
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.Unmarshal(payload, &parsed); err != nil {
		return "", errTranslateBad
	}
	if resp.StatusCode != http.StatusOK {
		if strings.TrimSpace(parsed.Error.Message) != "" {
			return "", errors.New("翻译失败：" + parsed.Error.Message)
		}
		return "", errTranslateDown
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errTranslateBad
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func callGoogleProxy(proxy, text string) (string, error) {
	client := explicitProxyClient(proxy)
	if client == nil {
		return "", errors.New("翻译失败：代理地址无效")
	}
	return callGoogleClient(client, text)
}

func callGoogleClient(client *http.Client, text string) (string, error) {
	if client == nil {
		return "", errTranslateDown
	}
	form := url.Values{}
	form.Set("q", text)
	endpoint := googleShotEndpoint + "?client=gtx&sl=en&tl=zh-CN&dt=t"
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errTranslateDown
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", translateUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", errTranslateDown
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", errTranslateDown
	}
	if resp.StatusCode != http.StatusOK {
		return "", errTranslateDown
	}
	translated, err := parseGoogleTranslations(payload)
	if err != nil {
		return "", errTranslateBad
	}
	return translated, nil
}

func callMyMemory(client *http.Client, text string) (string, error) {
	if client == nil {
		return "", errTranslateDown
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		chunks := chunkText(line, 450)
		for j, chunk := range chunks {
			if j > 0 {
				b.WriteByte(' ')
			}
			part, err := myMemoryOne(client, chunk)
			if err != nil {
				return "", err
			}
			b.WriteString(part)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", errTranslateBad
	}
	return out, nil
}

func myMemoryOne(client *http.Client, text string) (string, error) {
	endpoint := myMemoryEndpoint + "?q=" + url.QueryEscape(text) + "&langpair=en%7Czh-CN"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", errTranslateDown
	}
	req.Header.Set("User-Agent", translateUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", errTranslateDown
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", errTranslateDown
	}
	var parsed struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		ResponseStatus int `json:"responseStatus"`
	}
	if err = json.Unmarshal(payload, &parsed); err != nil {
		return "", errTranslateBad
	}
	translated := strings.TrimSpace(parsed.ResponseData.TranslatedText)
	if resp.StatusCode != http.StatusOK || parsed.ResponseStatus != http.StatusOK || translated == "" {
		return "", errTranslateBad
	}
	if strings.Contains(strings.ToUpper(translated), "QUERY LENGTH LIMIT EXCEEDED") {
		return "", errTranslateTooLong
	}
	return translated, nil
}

func chunkText(text string, maxBytes int) []string {
	if maxBytes < 32 {
		maxBytes = 32
	}
	if len(text) <= maxBytes {
		return []string{text}
	}
	words := strings.Fields(text)
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
	}
	for _, word := range words {
		for len(word) > maxBytes {
			flush()
			cut := maxBytes
			for cut > 0 && !utf8.ValidString(word[:cut]) {
				cut--
			}
			if cut == 0 {
				cut = 1
			}
			out = append(out, word[:cut])
			word = word[cut:]
		}
		if word == "" {
			continue
		}
		extra := len(word)
		if b.Len() > 0 {
			extra++
		}
		if b.Len() > 0 && b.Len()+extra > maxBytes {
			flush()
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(word)
	}
	flush()
	if len(out) == 0 {
		return []string{text[:maxBytes]}
	}
	return out
}
