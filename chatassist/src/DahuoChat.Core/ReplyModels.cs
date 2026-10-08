namespace DahuoChat;

public static class ReplyTags
{
    public static readonly string[] All =
    [
        "温暖体贴", "幽默化解", "真诚夸赞", "委婉拒绝", "轻松简短",
        "共情倾听", "给台阶", "得体坚持", "诚恳道歉", "积极推进"
    ];

    public static readonly HashSet<string> Set = new(All);
}

public static class Tones
{
    public static readonly string[] All = ["温暖", "谦和", "幽默", "正式"];
    public const string Default = "温暖";
}

public sealed class ReadInsight
{
    public string Emotion { get; set; } = "";
    public string Intent { get; set; } = "";
    public string Avoid { get; set; } = "";

    public string Line
    {
        get
        {
            var parts = new[] { Emotion, Intent }.Where(s => !string.IsNullOrWhiteSpace(s)).ToArray();
            return parts.Length == 0 ? "" : string.Join(" · ", parts);
        }
    }
}

public sealed class ReplyItem
{
    public string Tag { get; set; } = "";
    public string Text { get; set; } = "";
    public string Why { get; set; } = "";
}

public sealed class SuggestionResult
{
    public ReadInsight Read { get; set; } = new();
    public List<ReplyItem> Replies { get; set; } = [];
    public int Best { get; set; }
    public bool Retried { get; set; }
    public string? Error { get; set; }

    public bool Ok => Error is null && Replies.Count >= 3;
}

public enum SuggestAdjust
{
    None,
    Warmer,
    Shorter,
    AnotherBatch,
    Polish
}

public sealed class SuggestRequest
{
    public string Relation { get; set; } = "朋友";
    public string Persona { get; set; } = PromptBuilder.DefaultPersona;
    public string Goal { get; set; } = "";
    public IReadOnlyList<string> Points { get; set; } = [];
    public string Tone { get; set; } = Tones.Default;
    public int Count { get; set; } = 3;
    public IReadOnlyList<string> History { get; set; } = [];
    public string LastMessage { get; set; } = "";
    public string ContactName { get; set; } = "";
    public SuggestAdjust Adjust { get; set; }
    public IReadOnlyList<string> AvoidReplies { get; set; } = [];
    public string Draft { get; set; } = "";
}

public sealed class LlmOptions
{
    public string BaseUrl { get; set; } = AiPresets.DeepSeek.BaseUrl;
    public string ApiKey { get; set; } = "";
    public string Model { get; set; } = AiPresets.DeepSeek.Model;
    public TimeSpan Timeout { get; set; } = TimeSpan.FromSeconds(15);
}

public sealed record AiPreset(string Name, string BaseUrl, string Model, string Hint)
{
    public static readonly AiPreset[] All =
    [
        new("DeepSeek", "https://api.deepseek.com", "deepseek-chat", "推荐。国内直连"),
        new("智谱 GLM-4-Flash", "https://open.bigmodel.cn/api/paas/v4", "glm-4-flash", "有免费额度"),
        new("通义", "https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen-turbo", "阿里云"),
        new("Kimi", "https://api.moonshot.cn/v1", "moonshot-v1-8k", "月之暗面")
    ];

    public static AiPreset DeepSeek => All[0];
}

public static class AiPresets
{
    public static AiPreset DeepSeek => AiPreset.DeepSeek;
}
