using System.Text.RegularExpressions;
using Xunit.Abstractions;

namespace DahuoChat.Tests;

public class AccountBookTests
{
    [Fact]
    public void Remove_keeps_neighbors_and_roundtrips_the_file()
    {
        var dir = Path.Combine(Path.GetTempPath(), "dahuo-acc-" + Guid.NewGuid().ToString("N"));
        var path = Path.Combine(dir, "accounts.json");
        var file = new AccountFile
        {
            FirstRunDone = true,
            Accounts =
            [
                new AccountRecord { Id = "wechat", Platform = "wechat", Name = "微信", Sub = "海外号" },
                new AccountRecord { Id = "dy1", Platform = "douyin", Name = "抖音 1", Sub = "主号", Brand = "Edge" },
                new AccountRecord { Id = "dy2", Platform = "douyin", Name = "抖音 2", Sub = "店铺号", Brand = "Chrome" }
            ]
        };

        var rejected = AccountBook.Remove(file, "add");
        Assert.False(rejected.Ok);
        Assert.Equal(3, file.Accounts.Count);

        var missing = AccountBook.Remove(file, "nope");
        Assert.False(missing.Ok);
        Assert.Equal(3, file.Accounts.Count);

        var middle = AccountBook.Remove(file, "dy1");
        Assert.True(middle.Ok);
        Assert.Equal("dy2", middle.NextId);
        Assert.Equal("dy1", middle.Removed!.Id);
        Assert.Contains("不会发出任何消息", AccountBook.ConfirmText(middle.Removed));
        Assert.Contains("抖音 1 · 主号", AccountBook.ConfirmText(middle.Removed));
        Assert.Contains("独立登录文件夹", AccountBook.ConfirmText(middle.Removed));
        Assert.Contains("不会被卸载", AccountBook.ConfirmText(file.Accounts[0]));

        AccountBook.Save(path, file);
        var loaded = AccountBook.Load(path);
        Assert.True(loaded.FirstRunDone);
        Assert.Equal(new[] { "wechat", "dy2" }, loaded.Accounts.Select(a => a.Id).ToArray());
        Assert.Equal("Chrome", loaded.Accounts[1].Brand);

        var last = AccountBook.Remove(loaded, "dy2");
        Assert.Equal("wechat", last.NextId);
        var only = AccountBook.Remove(loaded, "wechat");
        Assert.Null(only.NextId);
        Assert.Empty(loaded.Accounts);
        AccountBook.Save(path, loaded);
        Assert.Empty(AccountBook.Load(path).Accounts);

        File.WriteAllText(path, "{");
        Assert.Empty(AccountBook.Load(path).Accounts);
        Directory.Delete(dir, recursive: true);
    }

    [Fact]
    public void Profile_directory_delete_removes_the_folder()
    {
        var dir = Path.Combine(Path.GetTempPath(), "dahuo-profile-" + Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(dir);
        File.WriteAllText(Path.Combine(dir, "profile.json"), "{}");
        Assert.True(AccountBook.TryDeleteDirectory(dir));
        Assert.False(Directory.Exists(dir));
        Assert.True(AccountBook.TryDeleteDirectory(dir));
        Assert.False(AccountBook.TryDeleteDirectory(" "));
    }
}

public class DemoSuggestionTests
{
    [Fact]
    public void Warmer_shorter_and_polish_are_different_replies()
    {
        var wechat = DemoSuggestions.Wechat()[0].Text;
        var warmer = DemoSuggestions.Pick(SuggestAdjust.Warmer)[0].Text;
        var shorter = DemoSuggestions.Pick(SuggestAdjust.Shorter)[0].Text;
        var polish = DemoSuggestions.Pick(SuggestAdjust.Polish)[0].Text;
        var batch = DemoSuggestions.Pick(SuggestAdjust.AnotherBatch)[1].Text;
        Assert.NotEqual(wechat, warmer);
        Assert.NotEqual(wechat, shorter);
        Assert.NotEqual(wechat, polish);
        Assert.NotEqual(warmer, shorter);
        Assert.Equal(DemoSuggestions.Wechat()[1].Text, batch);
        Assert.Contains("很暖", warmer);
    }
}

public class UiButtonTests
{
    private readonly ITestOutputHelper _output;

    public UiButtonTests(ITestOutputHelper output) => _output = output;

    [Fact]
    public void Every_button_has_a_handler_that_does_something()
    {
        var root = FindRoot();
        var app = Path.Combine(root, "src", "DahuoChat.App");
        var files = new[] { Path.Combine(app, "MainWindow.xaml") }
            .Concat(Directory.GetFiles(Path.Combine(app, "Views"), "*.xaml"))
            .ToList();
        var seen = new List<string>();
        foreach (var file in files)
        {
            var doc = System.Xml.Linq.XDocument.Load(file, System.Xml.Linq.LoadOptions.SetLineInfo);
            var buttons = doc.Descendants().Where(e => e.Name.LocalName == "Button").ToList();
            if (file.EndsWith("WeChatDemoView.xaml") || file.EndsWith("DouyinDemoView.xaml"))
            {
                Assert.Empty(buttons);
                var raw = File.ReadAllText(file);
                Assert.DoesNotContain("Click=", raw);
                seen.Add(Path.GetFileName(file) + " 没有按钮（示意画面）");
                continue;
            }
            Assert.NotEmpty(buttons);
            var code = File.ReadAllText(Path.ChangeExtension(file, ".xaml.cs"));
            if (file.EndsWith("MainWindow.xaml"))
                code += File.ReadAllText(Path.Combine(app, "Probe.cs"));
            var methods = Methods(code);
            foreach (var button in buttons)
            {
                var click = button.Attributes().FirstOrDefault(a => a.Name.LocalName == "Click")?.Value;
                var line = (button as System.Xml.IXmlLineInfo)?.LineNumber ?? 0;
                Assert.False(string.IsNullOrWhiteSpace(click), Path.GetFileName(file) + ":" + line + " 的按钮没有 Click");
                Assert.True(methods.TryGetValue(click!, out var body), click + " 没有方法体");
                Assert.False(string.IsNullOrWhiteSpace(body), click + " 是空的");
                Assert.DoesNotContain("NotImplemented", body);
                Assert.False(Regex.IsMatch(body.Trim(), @"^(return\s*;\s*)?$"), click + " 只有 return");
                seen.Add(Path.GetFileName(file) + " " + click);
            }
        }
        _output.WriteLine(string.Join(Environment.NewLine, seen));
        Assert.Contains(seen, row => row.Contains("Remove_Click"));
        Assert.Contains(seen, row => row.Contains("ConfirmRemove_Click"));
        Assert.Contains(seen, row => row.Contains("Account_Click"));
    }

    [Fact]
    public void Remove_asks_before_it_deletes_and_still_has_add()
    {
        var root = FindRoot();
        var app = Path.Combine(root, "src", "DahuoChat.App");
        var main = File.ReadAllText(Path.Combine(app, "MainWindow.xaml"));
        var code = File.ReadAllText(Path.Combine(app, "MainWindow.xaml.cs"));
        var session = File.ReadAllText(Path.Combine(app, "Session.cs"));
        var storage = File.ReadAllText(Path.Combine(app, "Storage.cs"));
        var wizard = File.ReadAllText(Path.Combine(app, "Views", "WizardView.xaml"));
        var settingsXaml = File.ReadAllText(Path.Combine(app, "Views", "SettingsView.xaml"));
        var settings = File.ReadAllText(Path.Combine(app, "Views", "SettingsView.xaml.cs"));
        var methods = Methods(code);
        var settingMethods = Methods(settings);

        Assert.Contains("Text=\"-\"", main);
        Assert.Contains("Click=\"Remove_Click\"", main);
        Assert.Contains("Id = \"add\"", session);
        Assert.Contains("Title = \"添加\"", session);
        Assert.DoesNotContain("RemoveAccount", methods["Remove_Click"]);
        Assert.Contains("ConfirmText", methods["Remove_Click"]);
        Assert.DoesNotContain("RemoveAccount", methods["CancelRemove_Click"]);
        Assert.Contains("RemoveAccount", methods["ConfirmRemove_Click"]);
        Assert.Contains("AccountBook.Remove", session);
        Assert.Contains("SaveAccounts", session);
        Assert.Contains("DouyinRoot", session);
        Assert.Contains("DemoSuggestions.Pick", session);
        Assert.Contains("Button.ClickEvent", File.ReadAllText(Path.Combine(app, "Probe.cs")));
        Assert.Contains("AccountBook.Load", storage);
        Assert.Contains("AccountBook.Save", storage);
        Assert.Contains("IsEnabled=\"{Binding CanGoBack}\"", wizard);
        Assert.Contains("CloseOverlay", Methods(File.ReadAllText(Path.Combine(app, "Views", "WizardView.xaml.cs")))["Back_Click"]);
        Assert.Contains("PasswordChanged=\"Key_Changed\"", settingsXaml);
        Assert.Contains("FlushKey", settingMethods["Key_Changed"]);
        Assert.Contains("ApiKey", settingMethods["FlushKey"]);
        Assert.Contains("FlushKey", settingMethods["Back_Click"]);
        Assert.Contains("先写上标题", settingMethods["Add_Click"]);
        Assert.Contains("WeChatLocator", methods["Launch_Click"]);
        Assert.Contains("UseDock", methods["BadView_Click"]);
        Assert.Contains("WindowState.Minimized", methods["Min_Click"]);
        Assert.Contains("Close()", methods["Close_Click"]);
        Assert.Contains("TestAiAsync", settingMethods["Test_Click"]);
        Assert.Contains("ShowDialog", settingMethods["Import_Click"]);
        Assert.Contains("ImportCsv", settingMethods["Import_Click"]);
        Assert.Contains("ShowDialog", settingMethods["Export_Click"]);
        Assert.Contains("ExportCsv", settingMethods["Export_Click"]);

        var tone = settingsXaml.IndexOf("Content=\"温暖\"", StringComparison.Ordinal);
        var modest = settingsXaml.IndexOf("Content=\"谦和\"", StringComparison.Ordinal);
        var funny = settingsXaml.IndexOf("Content=\"幽默\"", StringComparison.Ordinal);
        var formal = settingsXaml.IndexOf("Content=\"正式\"", StringComparison.Ordinal);
        Assert.True(tone >= 0 && tone < modest && modest < funny && funny < formal);
        Assert.Equal(new[] { "温暖", "谦和", "幽默", "正式" }, Tones.All);
    }

    private static Dictionary<string, string> Methods(string source)
    {
        var map = new Dictionary<string, string>();
        var rx = new Regex(
            @"private\s+(?:async\s+)?void\s+(?<name>\w+)\s*\([^)]*\)\s*(?:=>\s*(?<expr>[^;]+);|\{(?<body>.*?)\n    \})",
            RegexOptions.Singleline | RegexOptions.CultureInvariant);
        foreach (Match match in rx.Matches(source))
        {
            var body = match.Groups["expr"].Success ? match.Groups["expr"].Value : match.Groups["body"].Value;
            map[match.Groups["name"].Value] = body;
        }
        return map;
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
