using System.Net;
using System.Text;
using System.Text.Json;

namespace DahuoChat.Tests;

public class ReplyValidatorTests
{
    private static string Json(params (string tag, string text, string why)[] items)
    {
        var replies = string.Join(",", items.Select(i =>
            "{\"tag\":" + Q(i.tag) + ",\"text\":" + Q(i.text) + ",\"why\":" + Q(i.why) + "}"));
        return "{\"read\":{\"emotion\":\"委屈\",\"intent\":\"想被听见\",\"avoid\":\"别讲道理\"},\"replies\":[" + replies + "],\"best\":0}";
    }

    private static string Q(string s) => JsonSerializer.Serialize(s);

    [Fact]
    public void Accepts_three_unique_whitelisted_replies_and_truncates_why()
    {
        var why = new string('呀', 28);
        var raw = Json(
            ("共情倾听", "当着全组的面骂？这也太不给人留面子了", why),
            ("温暖体贴", "抱抱，被当众说最难受了", "接住委屈"),
            ("轻松简短", "辛苦了兄弟，想骂就骂，我听着", "不说教"));
        var outcome = ReplyValidator.Validate(raw, "好朋友");
        Assert.NotNull(outcome.Result);
        Assert.Equal(3, outcome.Result!.Replies.Count);
        Assert.Equal(20, TextUtil.Runes(outcome.Result.Replies[0].Why));
        Assert.Equal("委屈 · 想被听见", outcome.Result.Read.Line);
    }

    [Fact]
    public void Rejects_unknown_duplicate_and_long_text()
    {
        var longText = new string('啊', 61);
        Assert.Contains("未知标签", string.Join(" ", ReplyValidator.Validate(Json(
            ("推进", "好", "短"),
            ("温暖体贴", "好呀", "短"),
            ("轻松简短", "行", "短")), "朋友").Problems));
        Assert.Contains("标签重复", string.Join(" ", ReplyValidator.Validate(Json(
            ("温暖体贴", "好呀", "短"),
            ("温暖体贴", "也好", "短"),
            ("轻松简短", "行", "短")), "朋友").Problems));
        Assert.Contains("过长", string.Join(" ", ReplyValidator.Validate(Json(
            ("温暖体贴", longText, "短"),
            ("轻松简短", "行", "短"),
            ("共情倾听", "我听着", "短")), "朋友").Problems));
    }

    [Fact]
    public void Drops_greasy_words_and_keeps_lover_exception()
    {
        var greasy = ReplyValidator.Validate(Json(
            ("温暖体贴", "宝子你辛苦了", "短"),
            ("轻松简短", "我听着", "短"),
            ("共情倾听", "怎么了", "短")), "朋友");
        Assert.Null(greasy.Result);

        var kept = ReplyValidator.Validate(Json(
            ("温暖体贴", "宝子你辛苦了", "短"),
            ("轻松简短", "我听着", "短"),
            ("共情倾听", "怎么了", "短"),
            ("给台阶", "没事的", "短")), "朋友");
        Assert.NotNull(kept.Result);
        Assert.Equal(3, kept.Result!.Replies.Count);
        Assert.DoesNotContain(kept.Result.Replies, r => r.Text.Contains("宝子"));

        var lover = ReplyValidator.Validate(Json(
            ("温暖体贴", "亲爱的，我到了", "短"),
            ("轻松简短", "等你", "短"),
            ("真诚夸赞", "你今天这件外套好看", "短")), "恋人");
        Assert.NotNull(lover.Result);
        Assert.Contains("亲爱的", lover.Result!.Replies[0].Text);
    }

    [Fact]
    public void Extracts_json_wrapped_in_prose()
    {
        var raw = "好的\n" + Json(("温暖体贴", "在的", "短"), ("轻松简短", "嗯", "短"), ("共情倾听", "你说", "短")) + "\n完毕";
        Assert.NotNull(ReplyValidator.Validate(raw, "朋友").Result);
        Assert.Null(ReplyValidator.ExtractObject("没有括号"));
    }
}

public class PromptTests
{
    [Fact]
    public void Template_includes_goal_history_and_adjustments()
    {
        var text = PromptBuilder.BuildUser(new SuggestRequest
        {
            Relation = "好朋友",
            Goal = "周六去不了",
            Points = ["说清原因"],
            Tone = "温暖",
            Count = 3,
            History = ["对方：来啊"],
            LastMessage = "就等你了",
            Adjust = SuggestAdjust.AnotherBatch,
            AvoidReplies = ["下周见"]
        });
        Assert.Contains("周六去不了", text);
        Assert.Contains("对方：来啊", text);
        Assert.Contains("就等你了", text);
        Assert.Contains("【请避开】", text);
        Assert.Contains("下周见", text);
        Assert.Contains("温暖", text);

        var warmer = PromptBuilder.BuildUser(new SuggestRequest { LastMessage = "在吗", Adjust = SuggestAdjust.Warmer });
        Assert.Contains("【调整】请更温暖", warmer);
        var shorter = PromptBuilder.BuildUser(new SuggestRequest { LastMessage = "在吗", Adjust = SuggestAdjust.Shorter });
        Assert.Contains("【调整】请更简短", shorter);
        var polish = PromptBuilder.BuildUser(new SuggestRequest { Adjust = SuggestAdjust.Polish, Draft = "在深圳瞎混" });
        Assert.Contains("在深圳瞎混", polish);
        Assert.Contains("视角", PromptBuilder.SystemPrompt);
    }
}

public class OpenAiClientTests
{
    [Fact]
    public async Task Posts_direct_json_params_and_retries_once()
    {
        var calls = 0;
        string? body = null;
        var handler = new StubHandler(req =>
        {
            calls++;
            body = req.Content!.ReadAsStringAsync().Result;
            Assert.Equal(HttpMethod.Post, req.Method);
            Assert.EndsWith("/chat/completions", req.RequestUri!.AbsolutePath);
            var content = calls == 1
                ? "不是json"
                : "{\"read\":{\"emotion\":\"开心\",\"intent\":\"道谢\",\"avoid\":\"别冷淡\"},\"replies\":["
                  + "{\"tag\":\"温暖体贴\",\"text\":\"太好了\",\"why\":\"一起高兴\"},"
                  + "{\"tag\":\"真诚夸赞\",\"text\":\"你有心了\",\"why\":\"夸具体\"},"
                  + "{\"tag\":\"轻松简短\",\"text\":\"吃好就行\",\"why\":\"不客套\"}],\"best\":0}";
            var payload = JsonSerializer.Serialize(new
            {
                choices = new[] { new { message = new { content } } }
            });
            return new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(payload, Encoding.UTF8, "application/json")
            };
        });
        var client = new OpenAiClient(new LlmOptions
        {
            BaseUrl = "https://api.deepseek.com",
            ApiKey = "sk-test",
            Model = "deepseek-chat",
            Timeout = TimeSpan.FromSeconds(15)
        }, handler);
        var result = await client.SuggestAsync(new SuggestRequest { LastMessage = "谢啦", Relation = "朋友" });
        Assert.True(result.Ok);
        Assert.True(result.Retried);
        Assert.Equal(2, calls);
        Assert.NotNull(body);
        using var doc = JsonDocument.Parse(body!);
        Assert.Equal(0.8, doc.RootElement.GetProperty("temperature").GetDouble());
        Assert.Equal(0.95, doc.RootElement.GetProperty("top_p").GetDouble());
        Assert.Equal(900, doc.RootElement.GetProperty("max_tokens").GetInt32());
        Assert.Equal("json_object", doc.RootElement.GetProperty("response_format").GetProperty("type").GetString());
        Assert.Equal("deepseek-chat", doc.RootElement.GetProperty("model").GetString());
        Assert.False(OpenAiClient.CreateDirectHandler().UseProxy);
    }
}

internal sealed class StubHandler : HttpMessageHandler
{
    private readonly Func<HttpRequestMessage, HttpResponseMessage> _next;
    public StubHandler(Func<HttpRequestMessage, HttpResponseMessage> next) => _next = next;
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        => Task.FromResult(_next(request));
}
