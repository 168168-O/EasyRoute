using System.Reflection;
using System.Text;

namespace DahuoChat;

public static class PromptBuilder
{
    public const string DefaultPersona =
        "我待人热心、说话实在，不喜欢客套和油腻的话。跟朋友随意，跟长辈和领导有礼貌。偶尔用个表情，不用网络烂梗。";

    public static string SystemPrompt { get; } = Load("system_prompt.txt");
    public static string UserTemplate { get; } = Load("user_template.txt");

    public static string BuildUser(SuggestRequest req)
    {
        var n = Math.Clamp(req.Count, ReplyValidator.MinCount, ReplyValidator.MaxCount);
        var points = req.Points is { Count: > 0 }
            ? string.Join("\n", req.Points.Select(p => "- " + p.Trim()))
            : "（无）";
        var history = req.History is { Count: > 0 }
            ? string.Join("\n", req.History.Select(h => h.Trim()))
            : "（无）";
        var goal = string.IsNullOrWhiteSpace(req.Goal) ? "无" : req.Goal.Trim();
        var text = UserTemplate
            .Replace("{relation}", req.Relation?.Trim() ?? "朋友")
            .Replace("{persona}", string.IsNullOrWhiteSpace(req.Persona) ? DefaultPersona : req.Persona.Trim())
            .Replace("{goal}", goal)
            .Replace("{points}", points)
            .Replace("{styles}", string.IsNullOrWhiteSpace(req.Tone) ? "自动" : req.Tone.Trim())
            .Replace("{n}", n.ToString())
            .Replace("{history}", history)
            .Replace("{last}", req.LastMessage?.Trim() ?? "");

        var extra = new StringBuilder();
        if (req.Adjust == SuggestAdjust.AnotherBatch && req.AvoidReplies.Count > 0)
        {
            extra.Append("\n【请避开】不要重复这些已经给出的回复：\n");
            foreach (var line in req.AvoidReplies)
                extra.Append("- ").AppendLine(line.Trim());
        }
        if (req.Adjust == SuggestAdjust.Warmer)
            extra.Append("\n【调整】请更温暖一点，仍然守住我想表达的意思。\n");
        if (req.Adjust == SuggestAdjust.Shorter)
            extra.Append("\n【调整】请更简短，每条尽量一句话，意思不变。\n");
        if (req.Adjust == SuggestAdjust.Polish)
        {
            extra.Append("\n【调整】下面是我自己写的草稿。请改成 ")
                .Append(n)
                .Append(" 条更好听的说法，意思一点都不要变，不要加我没说的承诺：\n")
                .Append(req.Draft?.Trim())
                .Append('\n');
        }
        return extra.Length == 0 ? text : text + extra;
    }

    public static object BuildBody(SuggestRequest req, string model)
    {
        return new
        {
            model,
            temperature = 0.8,
            top_p = 0.95,
            max_tokens = 900,
            response_format = new { type = "json_object" },
            messages = new object[]
            {
                new { role = "system", content = SystemPrompt },
                new { role = "user", content = BuildUser(req) }
            }
        };
    }

    private static string Load(string file)
    {
        var asm = typeof(PromptBuilder).Assembly;
        var name = asm.GetManifestResourceNames().FirstOrDefault(n => n.EndsWith(file, StringComparison.Ordinal));
        if (name is null)
            throw new InvalidOperationException("缺少提示词文件 " + file);
        using var stream = asm.GetManifestResourceStream(name)!;
        using var reader = new StreamReader(stream);
        return reader.ReadToEnd();
    }
}
