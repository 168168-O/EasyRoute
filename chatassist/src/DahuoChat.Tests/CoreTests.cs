namespace DahuoChat.Tests;

public class IdentityTests
{
    [Theory]
    [InlineData(BrowserBrand.Edge, "131.0.2903.48")]
    [InlineData(BrowserBrand.Chrome, "131.0.2903.48")]
    [InlineData(BrowserBrand.Browser360, "131.0.2903.48")]
    [InlineData(BrowserBrand.Qq, "131.0.2903.48")]
    [InlineData(BrowserBrand.Sogou, "130.0.1.2")]
    public void Ua_uses_real_major_and_matching_hints(BrowserBrand brand, string version)
    {
        var a = UserAgentFactory.Create(brand, version);
        var b = UserAgentFactory.Create(brand, version);
        Assert.Equal(a.UserAgent, b.UserAgent);
        Assert.Contains("Chrome/" + a.Major + ".0.0.0", a.UserAgent);
        Assert.Equal(version.Split('.')[0], a.Major);
        Assert.Contains(a.Brands, x => x.Brand == "Chromium" && x.Version == a.Major);
        Assert.Contains(a.FullVersionList, x => x.Brand == "Chromium" && x.Version == version);
        var json = UserAgentFactory.ToCdpJson(a);
        Assert.Contains(a.UserAgent, json);
        Assert.Contains("\"platform\":\"Windows\"", json);
        Assert.DoesNotContain("baidu", a.UserAgent, StringComparison.OrdinalIgnoreCase);
        Assert.DoesNotContain("Baidu", json);
    }

    [Fact]
    public void Brand_suffixes_and_auto_assignment()
    {
        var edge = UserAgentFactory.Create(BrowserBrand.Edge, "132.0.0.0");
        Assert.Contains("Edg/132.0.0.0", edge.UserAgent);
        Assert.Contains(edge.Brands, b => b.Brand == "Microsoft Edge");
        Assert.DoesNotContain(edge.Brands, b => b.Brand == "Google Chrome");

        var chrome = UserAgentFactory.Create(BrowserBrand.Chrome, "132.0.9.1");
        var qh = UserAgentFactory.Create(BrowserBrand.Browser360, "132.0.9.1");
        Assert.Equal(chrome.UserAgent, qh.UserAgent);

        var qq = UserAgentFactory.Create(BrowserBrand.Qq, "132.0.0.0");
        Assert.Contains("QQBrowser/22.1.0", qq.UserAgent);
        Assert.True(qq.AdvancedWarning);

        var sogou = UserAgentFactory.Create(BrowserBrand.Sogou, "132.0.0.0");
        Assert.Contains("SE 2.X MetaSr 1.0", sogou.UserAgent);

        Assert.Equal(BrowserBrand.Chrome, IdentityAssigner.PickUnused([BrowserBrand.Edge]));
        Assert.Equal(BrowserBrand.Browser360, IdentityAssigner.PickUnused([BrowserBrand.Edge, BrowserBrand.Chrome]));
        var again = IdentityAssigner.PickUnused([BrowserBrand.Edge, BrowserBrand.Chrome, BrowserBrand.Browser360], salt: 1);
        Assert.Contains(again, BrowserBrands.Preferred);

        var profile = IdentityProfile.Create("d1", BrowserBrand.Edge);
        Assert.Equal(BrowserBrand.Edge, IdentityProfile.Create(profile.Id, profile.Brand).Brand);
    }
}

public class ToneAndScriptTests
{
    [Fact]
    public void Tone_is_remembered_per_contact()
    {
        var memory = new ToneMemory();
        Assert.Equal("温暖", memory.Get("微信", "阿杰"));
        memory.Set("微信", "阿杰", "幽默");
        memory.Set("抖音", "林小雨", "谦和");
        Assert.Equal("幽默", memory.Get("微信", "阿杰"));
        Assert.Equal("谦和", memory.Get("抖音", "林小雨"));
        Assert.Equal("温暖", memory.Get("微信", "妈"));
        var round = ToneMemory.FromJson(memory.ToJson());
        Assert.Equal("幽默", round.Get("微信", "阿杰"));
        round.RememberPerContact = false;
        Assert.Equal("温暖", round.Get("微信", "阿杰"));
        Assert.Equal("正式", ToneMemory.Normalize("正式"));
        Assert.Equal("温暖", ToneMemory.Normalize("随便"));
    }

    [Fact]
    public void Csv_roundtrip_and_points_match()
    {
        var lib = ScriptLibrary.CreateSeed();
        var csv = lib.ToCsv();
        Assert.StartsWith("\uFEFF分类,", csv);
        var back = ScriptLibrary.FromCsv(csv);
        Assert.Equal(lib.Items.Count, back.Items.Count);
        Assert.Contains(back.Items, i => i.Title == "朋友邀约去不了" && i.Points.Contains("说清真实原因"));

        var quoted = "分类,标题,触发词,要点,参考说法,别这样说\n拒绝,\"想说\"\"不\"\"\",来啊,\"谢谢|原因,具体\",\"参考\n一行\",\"别编\"\n";
        var parsed = ScriptLibrary.FromCsv(quoted);
        Assert.Equal("想说\"不\"", parsed.Items[0].Title);
        Assert.Contains("原因,具体", parsed.Items[0].Points);
        Assert.Contains("一行", parsed.Items[0].Sample);

        var points = lib.RelevantPoints("周六你可一定要来啊", "拒绝");
        Assert.Contains("说清真实原因", points);
        var jsonBack = ScriptLibrary.FromJson(lib.ToJson());
        Assert.Equal(lib.Items.Count, jsonBack.Items.Count);
    }
}

public class ParsePolicyTests
{
    [Fact]
    public void Left_bubbles_are_them_and_green_is_me()
    {
        OcrBlock[] blocks =
        [
            new("兄弟！我新房终于装好了", 40, 20, 180, 36, 255, 255, 255),
            new("恭喜恭喜", 420, 70, 120, 36, 149, 236, 105),
            new("你可一定要来啊", 40, 130, 200, 36, 255, 255, 255)
        ];
        var lines = ChatParser.Parse(blocks, 640);
        Assert.Equal(Speaker.Them, lines[0].Who);
        Assert.Equal(Speaker.Me, lines[1].Who);
        Assert.Equal(Speaker.Them, lines[2].Who);
        Assert.True(ChatParser.ShouldSuggest(lines));
        Assert.False(ChatParser.ShouldSuggest(lines.Take(2).ToList()));
    }

    [Fact]
    public void Thumbnail_hash_changes_when_pixels_change()
    {
        var a = Solid(20, 20, 10);
        var b = Solid(20, 20, 240);
        var ha = ThumbnailHash.Average(a, 20, 20, 20 * 4);
        var hb = ThumbnailHash.Average(b, 20, 20, 20 * 4);
        Assert.Equal(ha, ThumbnailHash.Average((byte[])a.Clone(), 20, 20, 80));
        Assert.True(ThumbnailHash.Changed(ha, hb));
    }

    [Fact]
    public void Observer_only_accepts_their_new_text()
    {
        Assert.True(ObserverMessages.TryParse("{\"type\":\"message\",\"who\":\"them\",\"text\":\"好久不见\"}", out var who, out var text, out _));
        Assert.Equal("them", who);
        Assert.Equal("好久不见", text);
        Assert.False(ObserverMessages.TryParse("{\"type\":\"message\",\"who\":\"me\",\"text\":\"好\"}", out _, out _, out _));
        Assert.True(ObserverMessages.TryParse("{\"type\":\"unread\",\"count\":3}", out _, out _, out var unread));
        Assert.Equal(3, unread);
    }

    [Fact]
    public void Sleep_focus_hosts_and_hotkey()
    {
        var now = DateTimeOffset.Parse("2026-10-08T12:00:00+08:00");
        var tabs = new[]
        {
            new TabSnapshot("a", true, now.AddMinutes(-11), null),
            new TabSnapshot("b", true, now.AddMinutes(-3), null),
            new TabSnapshot("e", true, now.AddMinutes(-2), null),
            new TabSnapshot("c", true, now.AddMinutes(-1), null),
            new TabSnapshot("d", false, now.AddHours(-1), now.AddMinutes(-4))
        };
        var actions = SleepPolicy.Tick(tabs, now);
        Assert.Contains(actions, a => a.Id == "a" && a.Kind == SleepActionKind.Suspend);
        Assert.Contains(actions, a => a.Id == "d" && a.Kind == SleepActionKind.Peek);
        Assert.True(actions.Count(a => a.Kind == SleepActionKind.Suspend) >= 2);

        Assert.True(EmbedHealth.ShouldFallback(false, 800, 600, 0, false));
        Assert.True(EmbedHealth.ShouldFallback(true, 10, 10, 0, false));
        Assert.True(EmbedHealth.ShouldFallback(true, 800, 600, 5, false));
        Assert.True(EmbedHealth.ShouldFallback(true, 800, 600, 0, true));
        Assert.False(EmbedHealth.ShouldFallback(true, 800, 600, 1, false));

        var (x, y) = ComposerFocus.SuggestPoint(900, 700);
        Assert.True(ComposerFocus.IsSafeComposerPoint(x, y, 900, 700));
        Assert.False(ComposerFocus.IsSafeComposerPoint(860, 680, 900, 700));

        Assert.True(DouyinHosts.IsAllowed("https://www.douyin.com/"));
        Assert.True(DouyinHosts.IsAllowed("https://lf3-cdn.douyinstatic.com/obj/a.js"));
        Assert.False(DouyinHosts.IsAllowed("https://example.com/"));
        Assert.False(DouyinHosts.IsAllowed("https://www.douyin.com.evil.test/"));

        Assert.True(HotkeySpec.TryParse("Ctrl+Alt+R", out var mods, out var key));
        Assert.Equal(0x0002u | 0x0001u, mods);
        Assert.Equal((uint)'R', key);
        Assert.True(HotkeySpec.Conflicts("Ctrl + Alt + R", "ctrl+alt+r"));
        Assert.False(HotkeySpec.TryParse("R", out _, out _));

        var paths = WeChatPaths.Candidates(["C:\\Program Files\\Tencent\\Weixin"], ["D:\\Apps"]).ToList();
        Assert.Contains(paths, p => p.EndsWith("Weixin.exe"));
        Assert.Contains(paths, p => p.Contains("WeChat.exe"));
    }

    private static byte[] Solid(int w, int h, byte gray)
    {
        var data = new byte[w * h * 4];
        for (var i = 0; i < data.Length; i += 4)
        {
            data[i] = gray;
            data[i + 1] = gray;
            data[i + 2] = gray;
            data[i + 3] = 255;
        }
        return data;
    }
}

public class SafetyScanTests
{
    [Fact]
    public void Sources_do_not_send_or_name_foreign_apps()
    {
        var root = FindRoot();
        var needles = new[]
        {
            "VK_" + "RETURN",
            "Send" + "Keys",
            "keybd_" + "event",
            "{EN" + "TER}",
            "Keys." + "Enter",
            "Keys." + "Return",
            "tik" + "tok",
            "tele" + "gram",
            "what" + "sapp",
            "face" + "book",
            "insta" + "gram"
        };
        var files = Directory.EnumerateFiles(root, "*.*", SearchOption.AllDirectories)
            .Where(f => f.EndsWith(".cs") || f.EndsWith(".js") || f.EndsWith(".xaml") || f.EndsWith(".json") || f.EndsWith(".md") || f.EndsWith(".py") || f.EndsWith(".yml"))
            .Where(f => !f.Contains($"{Path.DirectorySeparatorChar}bin{Path.DirectorySeparatorChar}") && !f.Contains($"{Path.DirectorySeparatorChar}obj{Path.DirectorySeparatorChar}"))
            .ToList();
        Assert.NotEmpty(files);
        var hits = new List<string>();
        foreach (var file in files)
        {
            var text = File.ReadAllText(file);
            foreach (var needle in needles)
            {
                if (text.Contains(needle, StringComparison.OrdinalIgnoreCase))
                    hits.Add(Path.GetRelativePath(root, file) + " :: " + needle);
            }
            if (file.EndsWith(".js") && text.Contains(".click(", StringComparison.OrdinalIgnoreCase))
                hits.Add(Path.GetRelativePath(root, file) + " :: click");
        }
        Assert.True(hits.Count == 0, string.Join("\n", hits));
    }

    private static string FindRoot()
    {
        var dir = new DirectoryInfo(AppContext.BaseDirectory);
        while (dir is not null)
        {
            if (File.Exists(Path.Combine(dir.FullName, "DahuoChat.sln"))) return dir.FullName;
            dir = dir.Parent;
        }
        throw new InvalidOperationException("找不到 chatassist 根目录");
    }
}
