package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type aiSettings struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
	Model    string `json:"model"`
	DeepLKey string `json:"deeplKey"`
}

func parseAISettings(raw string) (aiSettings, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return aiSettings{}, nil
	}
	var settings aiSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return aiSettings{}, err
	}
	settings.Provider = strings.ToLower(strings.TrimSpace(settings.Provider))
	settings.BaseURL = strings.TrimSpace(settings.BaseURL)
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.Model = strings.TrimSpace(settings.Model)
	settings.DeepLKey = strings.TrimSpace(settings.DeepLKey)
	return settings, nil
}

func (s aiSettings) enabled() bool {
	switch s.Provider {
	case "openai":
		return s.APIKey != "" && s.BaseURL != "" && s.Model != ""
	case "deepl":
		return s.DeepLKey != ""
	default:
		return false
	}
}

func isDomesticAIHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, suffix := range []string{"deepseek.com", "aliyuncs.com", "aliyun.com", "bigmodel.cn", "zhipuai.cn", "moonshot.cn"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func openAIChatURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(strings.ToLower(base), "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

func deeplEndpoint(key string) string {
	if strings.HasSuffix(strings.TrimSpace(key), ":fx") {
		return "https://api-free.deepl.com/v2/translate"
	}
	return "https://api.deepl.com/v2/translate"
}

func deeplLang(code string) string {
	switch code {
	case "zh-Hans", "zh-CN", "":
		return "ZH"
	case "zh-Hant":
		return "ZH-HANT"
	case "en":
		return "EN"
	case "ja":
		return "JA"
	case "ko":
		return "KO"
	case "fr":
		return "FR"
	case "de":
		return "DE"
	case "es":
		return "ES"
	case "ru":
		return "RU"
	case "pt":
		return "PT"
	case "vi":
		return "VI"
	default:
		return strings.ToUpper(code)
	}
}

func aiInstruction(to string) string {
	switch {
	case to == "en":
		return "译成自然的英文。"
	case to == "zh-Hant":
		return "译成自然的繁体中文。"
	case isChineseCode(to) || to == "":
		return "译成自然的简体中文。"
	default:
		return "译成 " + to + "。"
	}
}

const aiSystemPrompt = "你是聊天翻译。译得自然、准确。俚语和缩写要译出原来的意思，不要按字面硬译，例如 lol、ngl、gm、wyd、tbh。只返回译文，不要解释，不要加引号。"

func (e *translateEngine) aiHTTPClient(rawURL string) *http.Client {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	if isDomesticAIHost(parsed.Hostname()) {
		return e.direct
	}
	if e.core != nil {
		return e.core
	}
	if e.proxy != nil {
		return e.proxy
	}
	return nil
}

func (e *translateEngine) translateAI(ctx context.Context, settings aiSettings, text, from, to string) (TranslateResult, error) {
	if !settings.enabled() {
		return TranslateResult{}, errTranslateBad
	}
	if settings.Provider == "deepl" {
		return e.translateDeepL(ctx, settings.DeepLKey, text, from, to)
	}
	return e.translateOpenAI(ctx, settings, text, from, to)
}

func (e *translateEngine) translateOpenAI(ctx context.Context, settings aiSettings, text, from, to string) (TranslateResult, error) {
	endpoint := openAIChatURL(settings.BaseURL)
	client := e.aiHTTPClient(endpoint)
	if client == nil {
		return TranslateResult{}, errAINeedsProxy
	}
	payload := map[string]any{
		"model": settings.Model,
		"messages": []map[string]string{
			{"role": "system", "content": aiSystemPrompt + aiInstruction(to)},
			{"role": "user", "content": text},
		},
		"temperature": 0.2,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return TranslateResult{}, errTranslateBad
	}
	status, body, _, err := e.doClient(ctx, client, http.MethodPost, endpoint, string(raw), map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + settings.APIKey,
	})
	if err != nil {
		return TranslateResult{}, errTranslateDown
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return TranslateResult{}, errAIKeyRejected
	}
	if status != http.StatusOK {
		return TranslateResult{}, errTranslateBad
	}
	translated, err := parseOpenAIChat(body)
	if err != nil {
		return TranslateResult{}, err
	}
	return TranslateResult{
		Text:       translated,
		SourceLang: from,
		TargetLang: to,
		Provider:   "openai",
		Label:      settings.Model,
	}, nil
}

func parseOpenAIChat(body []byte) (string, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", errTranslateBad
	}
	if len(payload.Choices) == 0 {
		return "", errTranslateBad
	}
	text := strings.TrimSpace(payload.Choices[0].Message.Content)
	text = strings.Trim(text, "\"“”")
	if text == "" {
		return "", errTranslateBad
	}
	return text, nil
}

func (e *translateEngine) translateDeepL(ctx context.Context, key, text, from, to string) (TranslateResult, error) {
	endpoint := deeplEndpoint(key)
	client := e.aiHTTPClient(endpoint)
	if client == nil {
		return TranslateResult{}, errAINeedsProxy
	}
	form := url.Values{}
	form.Set("text", text)
	form.Set("target_lang", deeplLang(to))
	if from != "" {
		form.Set("source_lang", deeplLang(from))
	}
	status, body, _, err := e.doClient(ctx, client, http.MethodPost, endpoint, form.Encode(), map[string]string{
		"Content-Type":  "application/x-www-form-urlencoded",
		"Authorization": "DeepL-Auth-Key " + key,
	})
	if err != nil {
		return TranslateResult{}, errTranslateDown
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return TranslateResult{}, errAIKeyRejected
	}
	if status != http.StatusOK {
		return TranslateResult{}, errTranslateBad
	}
	var payload struct {
		Translations []struct {
			Text                   string `json:"text"`
			DetectedSourceLanguage string `json:"detected_source_language"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.Translations) == 0 {
		return TranslateResult{}, errTranslateBad
	}
	translated := strings.TrimSpace(payload.Translations[0].Text)
	if translated == "" {
		return TranslateResult{}, errTranslateBad
	}
	detected := from
	if detected == "" {
		detected = strings.ToLower(payload.Translations[0].DetectedSourceLanguage)
	}
	return TranslateResult{
		Text:       translated,
		SourceLang: detected,
		TargetLang: to,
		Provider:   "deepl",
		Label:      "DeepL",
	}, nil
}

var (
	errAIKeyRejected = errString("API Key 没有通过，请检查密钥和接口地址。")
	errAINeedsProxy  = errString("国外的模型要走代理。请先连接，再试。")
)

type errString string

func (e errString) Error() string { return string(e) }
