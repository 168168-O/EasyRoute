using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;

namespace DahuoChat;

public sealed class OpenAiClient
{
    private readonly HttpClient _http;
    private readonly LlmOptions _options;

    public OpenAiClient(LlmOptions options, HttpMessageHandler? handler = null)
    {
        _options = options;
        handler ??= CreateDirectHandler();
        _http = new HttpClient(handler, disposeHandler: handler is not SocketsHttpHandler ? false : true)
        {
            Timeout = options.Timeout <= TimeSpan.Zero ? TimeSpan.FromSeconds(15) : options.Timeout
        };
    }

    public static SocketsHttpHandler CreateDirectHandler() => new()
    {
        UseProxy = false,
        Proxy = null,
        ConnectTimeout = TimeSpan.FromSeconds(10)
    };

    public async Task<SuggestionResult> SuggestAsync(SuggestRequest req, CancellationToken ct = default)
    {
        SuggestionResult? last = null;
        for (var attempt = 0; attempt < 2; attempt++)
        {
            string raw;
            try
            {
                raw = await CompleteAsync(PromptBuilder.BuildUser(req), 900, ct).ConfigureAwait(false);
            }
            catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException or InvalidOperationException)
            {
                return new SuggestionResult { Error = Friendly(ex), Retried = attempt > 0 };
            }

            var outcome = ReplyValidator.Validate(raw, req.Relation);
            if (outcome.Result is { } ok)
            {
                ok.Retried = attempt > 0;
                return ok;
            }
            last = new SuggestionResult
            {
                Error = string.Join("；", outcome.Problems),
                Retried = attempt > 0
            };
        }
        return last ?? new SuggestionResult { Error = "没有得到可用的回复" };
    }

    public async Task<string> TestAsync(CancellationToken ct = default)
    {
        var raw = await CompleteAsync("只回复两个字：好的", 32, ct).ConfigureAwait(false);
        return string.IsNullOrWhiteSpace(raw) ? "通了，但没有返回文字" : raw.Trim();
    }

    public async Task<string> CompleteAsync(string user, int maxTokens, CancellationToken ct)
    {
        if (string.IsNullOrWhiteSpace(_options.ApiKey))
            throw new InvalidOperationException("还没有填写 API Key");
        if (string.IsNullOrWhiteSpace(_options.BaseUrl))
            throw new InvalidOperationException("还没有填写接口地址");

        var url = _options.BaseUrl.TrimEnd('/') + "/chat/completions";
        var payload = JsonSerializer.Serialize(new
        {
            model = _options.Model,
            temperature = 0.8,
            top_p = 0.95,
            max_tokens = maxTokens,
            response_format = new { type = "json_object" },
            messages = new object[]
            {
                new { role = "system", content = maxTokens < 100 ? "你是连接测试。按用户的要求回答。" : PromptBuilder.SystemPrompt },
                new { role = "user", content = user }
            }
        });
        using var msg = new HttpRequestMessage(HttpMethod.Post, url);
        msg.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _options.ApiKey.Trim());
        msg.Content = new StringContent(payload, Encoding.UTF8, "application/json");
        using var resp = await _http.SendAsync(msg, ct).ConfigureAwait(false);
        var body = await resp.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        if (!resp.IsSuccessStatusCode)
            throw new HttpRequestException("接口返回 " + (int)resp.StatusCode + "：" + TrimErr(body));
        using var doc = JsonDocument.Parse(body);
        if (!doc.RootElement.TryGetProperty("choices", out var choices) || choices.GetArrayLength() == 0)
            throw new InvalidOperationException("接口没有返回内容");
        var content = choices[0].GetProperty("message").GetProperty("content").GetString();
        return content ?? "";
    }

    private static string TrimErr(string body)
    {
        var t = body.Replace('\n', ' ').Trim();
        return t.Length <= 180 ? t : t[..180];
    }

    private static string Friendly(Exception ex)
    {
        if (ex is TaskCanceledException) return "连接超时了（15 秒）。请检查网络，这个请求不走代理。";
        var msg = ex.Message;
        if (msg.Contains("API Key", StringComparison.Ordinal)) return "还没填钥匙。到设置里填上 DeepSeek 的 API Key 再试。";
        return "连不上 AI：" + msg;
    }
}
