using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Threading;

namespace DahuoChat;

public partial class MainWindow
{
    private async Task RunProbe()
    {
        var path = Path.Combine(_probeDir!, "probe.txt");
        try
        {
            Directory.CreateDirectory(_probeDir!);
            WindowState = WindowState.Normal;
            Width = 1366;
            Height = 768;
            Left = 40;
            Top = 40;
            await Dispatcher.InvokeAsync(() => { }, DispatcherPriority.ApplicationIdle);
            UpdateLayout();
            var lines = new List<string>();
            RunSteps(lines);
            File.WriteAllLines(path, lines);
            var failed = lines.Exists(line => line.StartsWith("FAIL ", StringComparison.Ordinal));
            Application.Current.Shutdown(failed ? 1 : 0);
        }
        catch (Exception ex)
        {
            try { File.WriteAllText(path, "FAIL 探针异常" + Environment.NewLine + ex); }
            catch { /* the exit code still reports it */ }
            Application.Current.Shutdown(1);
        }
    }

    private void RunSteps(List<string> log)
    {
        void Step(string name, Action action)
        {
            try
            {
                action();
                UpdateLayout();
                log.Add("ok " + name);
            }
            catch (Exception ex)
            {
                log.Add("FAIL " + name + " :: " + ex.Message);
            }
        }

        Step("侧栏有减，添加还在", () =>
        {
            _ = Named("RemoveAccountButton");
            _ = AccountButton("add");
            if (RemoveConfirm.Visibility == Visibility.Visible) throw new InvalidOperationException("确认框不该先出现");
            if (Session.AccountsFile.Accounts.Count != 4) throw new InvalidOperationException("演示账号不是 4 个");
        });

        Step("点账号会切换", () =>
        {
            Click(AccountButton("dy2"));
            if (!Session.TitleAccount.Contains("抖音 2", StringComparison.Ordinal))
                throw new InvalidOperationException(Session.TitleAccount);
            Click(AccountButton("wechat"));
            if (!Session.ShowWechat) throw new InvalidOperationException(Session.TitleAccount);
        });

        Step("展开和收起建议", () =>
        {
            if (Session.VisibleReplies.Count != 3) throw new InvalidOperationException("一开始应是 3 条");
            Click(RequireContent(text => text.StartsWith("还有", StringComparison.Ordinal)));
            UpdateLayout();
            if (!Session.Expanded || Session.VisibleReplies.Count != 5) throw new InvalidOperationException("没有展开");
            Click(RequireContent("收起"));
            if (Session.Expanded || Session.VisibleReplies.Count != 3) throw new InvalidOperationException("没有收起");
        });

        Step("语气按钮", () =>
        {
            Click(RequireTag("幽默"));
            if (Session.Tone != "幽默") throw new InvalidOperationException(Session.Tone);
        });

        Step("填入和复制", () =>
        {
            string? got = null;
            bool? paste = null;
            void OnDeliver(string text, bool flag) { got = text; paste = flag; }
            Session.Deliver += OnDeliver;
            try
            {
                Click(RequireContent("填入"));
                if (paste != true || got != Session.TextAt(0)) throw new InvalidOperationException("填入 " + paste + " " + got);
                got = null;
                paste = null;
                Click(RequireContent("复制"));
                if (paste != false || got != Session.TextAt(0)) throw new InvalidOperationException("复制 " + paste + " " + got);
            }
            finally
            {
                Session.Deliver -= OnDeliver;
            }
        });

        Step("话术抽屉", () =>
        {
            if (Session.DrawerOpen) Click(RequireContent("话术"));
            Click(RequireContent("话术"));
            UpdateLayout();
            if (!Session.DrawerOpen) throw new InvalidOperationException("抽屉没打开");
            var use = AllButtons().FirstOrDefault(b => Label(b).Contains("朋友邀约", StringComparison.Ordinal));
            if (use is null) throw new InvalidOperationException("没有话术条目");
            Click(use);
            if (!Session.Goal.Contains("说清真实原因", StringComparison.Ordinal)) throw new InvalidOperationException(Session.Goal);
            if (Session.DrawerOpen) throw new InvalidOperationException("用完应该收起");
        });

        Step("更温暖、更简短、换一批", () =>
        {
            var before = FirstReply();
            Click(RequireContent("更温暖"));
            if (FirstReply() == before || FirstReply() != DemoSuggestions.Warmer()[0].Text)
                throw new InvalidOperationException(FirstReply());
            Click(RequireContent("更简短"));
            if (FirstReply() != DemoSuggestions.Shorter()[0].Text) throw new InvalidOperationException(FirstReply());
            Click(RequireContent("换一批"));
            if (FirstReply() != DemoSuggestions.Wechat()[1].Text) throw new InvalidOperationException(FirstReply());
        });

        Step("回复建议和润色", () =>
        {
            Click(RequireContent("润色我的话"));
            if (!Session.PolishMode) throw new InvalidOperationException("没进润色");
            Click(RequireContent("回复建议"));
            if (Session.PolishMode) throw new InvalidOperationException("没回到建议");
        });

        Step("收起再打开右栏", () =>
        {
            Click(RequireContent("»"));
            if (Session.PanelOpen) throw new InvalidOperationException("没有收起");
            Click(RequireContent("«"));
            if (!Session.PanelOpen) throw new InvalidOperationException("没有打开");
        });

        Step("话术库和设置都能打开", () =>
        {
            Click(Named("ScriptsButton"));
            if (!Session.ShowSettings) throw new InvalidOperationException("话术库没打开设置");
            Click(Named("SettingsButton"));
            if (!Session.ShowSettings) throw new InvalidOperationException("设置没打开");
        });

        Step("空标题不会加话术", () =>
        {
            var before = Session.Scripts.Items.Count;
            TipBox("标题").Text = "  ";
            TipBox("要点，一行一个").Text = "";
            Click(RequireContent(text => text.Contains("加一条", StringComparison.Ordinal)));
            if (Session.Scripts.Items.Count != before) throw new InvalidOperationException("空标题不该加进去");
            if (!ToastText.Text.Contains("标题", StringComparison.Ordinal)) throw new InvalidOperationException(ToastText.Text);
        });

        Step("加一条话术", () =>
        {
            var before = Session.Scripts.Items.Count;
            TipBox("标题").Text = "探针话术";
            TipBox("要点，一行一个").Text = "说清楚";
            Click(RequireContent(text => text.Contains("加一条", StringComparison.Ordinal)));
            if (Session.Scripts.Items.Count != before + 1) throw new InvalidOperationException("没有加上");
            if (!Session.Scripts.Items.Any(item => item.Title == "探针话术")) throw new InvalidOperationException("列表里没有");
        });

        Step("话术分类", () =>
        {
            Click(In(SettingsPage, "问候"));
            if (Session.ScriptCategory != "问候") throw new InvalidOperationException(Session.ScriptCategory);
        });

        Step("AI 服务按钮", () =>
        {
            Click(In(SettingsPage, "通义"));
            if (Session.Settings.Model != "qwen-turbo") throw new InvalidOperationException(Session.Settings.Model);
            Click(In(SettingsPage, "DeepSeek"));
            if (Session.Settings.Model != "deepseek-chat") throw new InvalidOperationException(Session.Settings.Model);
        });

        Step("默认语气下拉框", () =>
        {
            var box = ToneCombo();
            box.SelectedIndex = 2;
            if (Session.Settings.DefaultTone != "幽默") throw new InvalidOperationException(Session.Settings.DefaultTone);
            box.SelectedIndex = 0;
            if (Session.Settings.DefaultTone != "温暖") throw new InvalidOperationException(Session.Settings.DefaultTone);
        });

        Step("返回刚才的聊天", () =>
        {
            Click(RequireContent("返回聊天"));
            if (!Session.ShowWechat) throw new InvalidOperationException(Session.TitleAccount);
        });

        Step("上一步回到刚才的账号", () =>
        {
            Click(AccountButton("add"));
            if (!Session.ShowWizard) throw new InvalidOperationException("没有打开添加");
            if (!BackButton().IsEnabled) throw new InvalidOperationException("有账号时上一步应能点");
            Click(BackButton());
            if (!Session.ShowWechat) throw new InvalidOperationException(Session.TitleAccount);
        });

        Step("向导里的平台、浏览器和名字", () =>
        {
            Click(AccountButton("add"));
            var wechat = In(WizardPage, "wechat");
            var douyin = In(WizardPage, "douyin");
            Click(wechat);
            if (Session.WizardPlatform != "wechat" || Session.WizardName != "海外号")
                throw new InvalidOperationException(Session.WizardPlatform + " " + Session.WizardName);
            Click(douyin);
            UpdateLayout();
            if (Session.WizardPlatform != "douyin" || Session.WizardName != "店铺号")
                throw new InvalidOperationException(Session.WizardPlatform + " " + Session.WizardName);
            if (Paint(wechat.Background) == Paint(douyin.Background))
                throw new InvalidOperationException("平台按钮看不出选中");
            var edge = In(WizardPage, "Edge");
            Click(edge);
            UpdateLayout();
            if (Session.WizardBrand != "Edge") throw new InvalidOperationException(Session.WizardBrand);
            if (Paint(edge.Background) == Paint(In(WizardPage, "Chrome").Background))
                throw new InvalidOperationException("浏览器按钮看不出选中");
            Click(In(WizardPage, "主号"));
            if (Session.WizardName != "主号") throw new InvalidOperationException(Session.WizardName);
        });

        Step("完成仍会添加账号", () =>
        {
            var before = Session.AccountsFile.Accounts.Count;
            Click(RequireContent("完成"));
            if (Session.AccountsFile.Accounts.Count != before + 1) throw new InvalidOperationException("没有加上");
            if (!Session.TitleAccount.Contains("主号", StringComparison.Ordinal)) throw new InvalidOperationException(Session.TitleAccount);
        });

        Step("减先确认，取消不删除", () =>
        {
            var id = Session.SelectedAccountId ?? throw new InvalidOperationException("没有选中账号");
            var before = Session.AccountsFile.Accounts.Count;
            Click(Named("RemoveAccountButton"));
            if (RemoveConfirm.Visibility != Visibility.Visible) throw new InvalidOperationException("没有确认");
            if (!RemoveConfirmText.Text.Contains("不会发出任何消息", StringComparison.Ordinal))
                throw new InvalidOperationException(RemoveConfirmText.Text);
            if (Session.AccountsFile.Accounts.Count != before) throw new InvalidOperationException("还没确定就删了");
            Click(Named("CancelRemoveButton"));
            if (RemoveConfirm.Visibility == Visibility.Visible) throw new InvalidOperationException("取消没关掉");
            if (!Session.AccountsFile.Accounts.Any(a => a.Id == id)) throw new InvalidOperationException("取消把账号删了");
        });

        Step("确定后从列表移除", () =>
        {
            var guard = 0;
            while (Session.AccountsFile.Accounts.Count > 0)
            {
                if (++guard > 8) throw new InvalidOperationException("移除停不下来");
                var id = Session.SelectedAccountId ?? throw new InvalidOperationException("没有选中账号");
                var before = Session.AccountsFile.Accounts.Count;
                Click(Named("RemoveAccountButton"));
                if (RemoveConfirm.Visibility != Visibility.Visible) throw new InvalidOperationException("没有确认 " + id);
                Click(Named("ConfirmRemoveButton"));
                if (Session.AccountsFile.Accounts.Any(a => a.Id == id)) throw new InvalidOperationException("还在 " + id);
                if (Session.AccountsFile.Accounts.Count != before - 1) throw new InvalidOperationException("数量没变");
            }
            if (!Session.ShowWizard) throw new InvalidOperationException("删光后应回到添加");
            UpdateLayout();
            if (Session.CanGoBack || BackButton().IsEnabled) throw new InvalidOperationException("没有账号时上一步不该能点");
            _ = AccountButton("add");
        });

        Step("没选中账号时减会提示", () =>
        {
            Click(Named("RemoveAccountButton"));
            if (RemoveConfirm.Visibility == Visibility.Visible) throw new InvalidOperationException("不该直接确认");
            if (!ToastText.Text.Contains("先在上面点", StringComparison.Ordinal)) throw new InvalidOperationException(ToastText.Text);
        });

        Step("最大化再还原", () =>
        {
            var before = WindowState;
            Click(RequireContent("□"));
            if (WindowState == before) throw new InvalidOperationException("窗口状态没变");
            Click(RequireContent("□"));
            if (WindowState != before) throw new InvalidOperationException("没有还原");
        });
    }

    private string FirstReply() => Session.VisibleReplies.Count == 0 ? "" : Session.VisibleReplies[0].Text;

    private static string Paint(System.Windows.Media.Brush? brush)
        => brush is System.Windows.Media.SolidColorBrush solid ? solid.Color.ToString() : brush?.GetType().Name ?? "";

    private Button Named(string name)
    {
        if (FindName(name) is Button button) return button;
        throw new InvalidOperationException("找不到 " + name);
    }

    private Button BackButton() => RequireContent("上一步");

    private ComboBox ToneCombo()
    {
        if (SettingsPage.FindName("ToneBox") is ComboBox box) return box;
        throw new InvalidOperationException("找不到语气下拉框");
    }

    private TextBox TipBox(string tip)
    {
        var found = FindVisual<TextBox>(SettingsPage).Where(box => (box.ToolTip as string) == tip).ToList();
        if (found.Count != 1) throw new InvalidOperationException(tip + " 文本框 " + found.Count);
        return found[0];
    }

    private Button AccountButton(string id)
    {
        var matches = AllButtons().Where(button => (button.Tag as string) == id && !IsInside(button, WizardPage) && !IsInside(button, SettingsPage)).ToList();
        if (matches.Count != 1) throw new InvalidOperationException(id + " 账号按钮 " + matches.Count);
        return matches[0];
    }

    private Button In(DependencyObject root, string tag)
    {
        var matches = AllButtons(root).Where(button => (button.Tag as string) == tag).ToList();
        if (matches.Count != 1) throw new InvalidOperationException(tag + " 按钮 " + matches.Count);
        return matches[0];
    }

    private Button RequireTag(string tag) => In(this, tag);

    private Button RequireContent(string text) => RequireContent(value => value == text);

    private Button RequireContent(Func<string, bool> match)
    {
        var found = AllButtons().Where(button =>
        {
            var text = button.Content as string ?? "";
            return match(text);
        }).ToList();
        if (found.Count == 0)
        {
            var have = string.Join(" | ", AllButtons().Select(button => button.Content as string ?? Label(button)).Take(50));
            throw new InvalidOperationException("没有匹配的按钮。现有：" + have);
        }
        return found[0];
    }

    private static void Click(Button button) => button.RaiseEvent(new RoutedEventArgs(Button.ClickEvent));

    private IEnumerable<Button> AllButtons() => AllButtons(this);

    private static IEnumerable<Button> AllButtons(DependencyObject root) => FindVisual<Button>(root);

    private static bool IsInside(DependencyObject child, DependencyObject parent)
    {
        for (DependencyObject? node = child; node is not null; node = VisualTreeHelper.GetParent(node))
        {
            if (ReferenceEquals(node, parent)) return true;
        }
        return false;
    }

    private static string Label(Button button)
    {
        if (button.Content is string text) return text;
        return string.Concat(FindVisual<TextBlock>(button).Select(block => block.Text));
    }

    private static IEnumerable<T> FindVisual<T>(DependencyObject root) where T : DependencyObject
    {
        var count = VisualTreeHelper.GetChildrenCount(root);
        for (var i = 0; i < count; i++)
        {
            var child = VisualTreeHelper.GetChild(root, i);
            if (child is T match) yield return match;
            foreach (var nested in FindVisual<T>(child)) yield return nested;
        }
    }
}
