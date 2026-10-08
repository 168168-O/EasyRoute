using System.Text.Json;

namespace DahuoChat;

public static class GreasyWords
{
    public static readonly string[] All =
    [
        "宝子", "亲爱的", "家人们", "绝绝子", "yyds", "YYDS",
        "么么哒", "小可爱", "宝贝", "你真优秀", "你太棒了", "亲~", "亲，", "亲,"
    ];

    public static bool IsLover(string? relation)
    {
        if (string.IsNullOrWhiteSpace(relation)) return false;
        string[] marks = ["恋人", "爱人", "老公", "老婆", "男朋友", "女朋友", "男票", "女票"];
        return marks.Any(relation.Contains);
    }

    public static IEnumerable<string> Hits(string text, string? relation)
    {
        if (string.IsNullOrEmpty(text)) yield break;
        var lover = IsLover(relation);
        foreach (var word in All)
        {
            if (lover && word == "亲爱的") continue;
            if (text.Contains(word, StringComparison.OrdinalIgnoreCase))
                yield return word;
        }
    }
}

public static class ReplyValidator
{
    public const int MinCount = 3;
    public const int MaxCount = 5;
    public const int MaxText = 60;
    public const int MaxWhy = 20;

    public sealed class Outcome
    {
        public SuggestionResult? Result { get; init; }
        public List<string> Problems { get; init; } = [];
        public bool ShouldRetry => Result is null;
    }

    public static Outcome Validate(string raw, string? relation)
    {
        var problems = new List<string>();
        JsonDocument doc;
        try
        {
            var json = ExtractObject(raw);
            if (json is null)
            {
                problems.Add("没有找到 JSON");
                return new Outcome { Problems = problems };
            }
            doc = JsonDocument.Parse(json);
        }
        catch (Exception ex)
        {
            problems.Add("JSON 解析失败:" + ex.Message);
            return new Outcome { Problems = problems };
        }

        using (doc)
        {
            var root = doc.RootElement;
            var read = new ReadInsight();
            if (root.TryGetProperty("read", out var rd) && rd.ValueKind == JsonValueKind.Object)
            {
                read.Emotion = Str(rd, "emotion");
                read.Intent = Str(rd, "intent");
                read.Avoid = Str(rd, "avoid");
            }

            if (!root.TryGetProperty("replies", out var arr) || arr.ValueKind != JsonValueKind.Array)
            {
                problems.Add("缺少 replies");
                return new Outcome { Problems = problems };
            }

            var parsed = new List<ReplyItem>();
            foreach (var el in arr.EnumerateArray())
            {
                var tag = Str(el, "tag");
                var text = Str(el, "text").Trim();
                var why = TextUtil.TruncateRunes(Str(el, "why"), MaxWhy);
                parsed.Add(new ReplyItem { Tag = tag, Text = text, Why = why });
            }

            if (parsed.Count < MinCount || parsed.Count > MaxCount)
                problems.Add("条数=" + parsed.Count);
            foreach (var item in parsed)
            {
                if (!ReplyTags.Set.Contains(item.Tag)) problems.Add("未知标签:" + item.Tag);
                if (TextUtil.Runes(item.Text) > MaxText) problems.Add("过长(" + TextUtil.Runes(item.Text) + "字)");
                if (string.IsNullOrWhiteSpace(item.Text)) problems.Add("空回复");
            }
            var tags = parsed.Select(p => p.Tag).ToList();
            if (tags.Count != tags.Distinct().Count()) problems.Add("标签重复");
            if (problems.Count > 0) return new Outcome { Problems = problems };

            var kept = parsed.Where(item => !GreasyWords.Hits(item.Text, relation).Any()).ToList();
            if (kept.Count < parsed.Count) problems.Add("油腻词");
            if (kept.Count < MinCount)
            {
                problems.Add("条数=" + kept.Count);
                return new Outcome { Problems = problems };
            }

            var best = 0;
            if (root.TryGetProperty("best", out var b) && b.TryGetInt32(out var bi) && bi >= 0 && bi < kept.Count)
                best = bi;

            return new Outcome
            {
                Problems = problems,
                Result = new SuggestionResult
                {
                    Read = read,
                    Replies = kept,
                    Best = best
                }
            };
        }
    }

    public static string? ExtractObject(string? raw)
    {
        if (string.IsNullOrWhiteSpace(raw)) return null;
        var start = raw.IndexOf('{');
        if (start < 0) return null;
        var depth = 0;
        var inStr = false;
        var esc = false;
        for (var i = start; i < raw.Length; i++)
        {
            var c = raw[i];
            if (inStr)
            {
                if (esc) esc = false;
                else if (c == '\\') esc = true;
                else if (c == '"') inStr = false;
                continue;
            }
            if (c == '"') { inStr = true; continue; }
            if (c == '{') depth++;
            else if (c == '}')
            {
                depth--;
                if (depth == 0) return raw[start..(i + 1)];
            }
        }
        return null;
    }

    private static string Str(JsonElement el, string name)
    {
        if (!el.TryGetProperty(name, out var v)) return "";
        return v.ValueKind switch
        {
            JsonValueKind.String => v.GetString() ?? "",
            JsonValueKind.Number => v.ToString(),
            _ => ""
        };
    }
}
