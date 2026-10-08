using System.Collections.ObjectModel;
using System.Windows.Media;

namespace DahuoChat;

public sealed class Session : NotifyBase
{
    private readonly bool _persist;
    public bool Demo { get; }
    public PersistedSettings Settings { get; private set; }
    public ToneMemory Tones { get; private set; }
    public ScriptLibrary Scripts { get; private set; }
    public AccountFile AccountsFile { get; private set; }

    public ObservableCollection<AccountVm> Accounts { get; } = [];
    public ObservableCollection<ReplyVm> VisibleReplies { get; } = [];
    public ObservableCollection<ScriptVm> VisibleScripts { get; } = [];
    public string[] Categories { get; } = ScriptCategories.All;

    private readonly List<ReplyItem> _all = [];
    private readonly List<string> _history = [];
    private string _lastTheir = "";
    private string _platform = "微信";
    private string _contact = "阿杰";
    private bool _expanded;
    private ScreenKind _screen = ScreenKind.Wechat;
    private string _tone = "温暖";
    private string _goal = "";
    private string _quote = "";
    private string _emotion = "";
    private string _contactLine = "";
    private string _initial = "杰";
    private string _memoryLine = "";
    private string _status = "只给建议，发送请你自己按";
    private string _titleAccount = "微信 · 海外号";
    private string _titleState = "已放入 · 微信本身没改动";
    private string _extraChip = "新消息自动出建议";
    private bool _polishMode;
    private string _draft = "";
    private string _scriptCategory = "拒绝";
    private bool _drawer;
    private bool _panelOpen = true;
    private string _aiState = "还没连接";
    private bool _noKey;
    private string _wizardPlatform = "douyin";
    private string _wizardBrand = "Auto";
    private string _wizardName = "店铺号";
    private string _wizardNote = "";
    private string _wizardWarn = "";
    private string _launchHint = "";
    private bool _wechatMissing;
    private string _envLine = "独立环境 · 抖音 1    浏览器 Edge（固定）    北京时间    国内直连";

    public Session(bool demo)
    {
        Demo = demo;
        _persist = !demo;
        if (demo)
        {
            Settings = new PersistedSettings { ApiKeyProtected = "demo" };
            Tones = new ToneMemory();
            Scripts = ScriptLibrary.CreateSeed();
            AccountsFile = DemoAccounts();
            ApiKey = "demo";
        }
        else
        {
            Settings = LocalStore.LoadSettings();
            Tones = LocalStore.LoadTones();
            Scripts = LocalStore.LoadScripts();
            if (Scripts.Items.Count == 0) Scripts = ScriptLibrary.CreateSeed();
            AccountsFile = LocalStore.LoadAccounts();
            ApiKey = SecretBox.Unprotect(Settings.ApiKeyProtected);
        }
        Tones.DefaultTone = Settings.DefaultTone;
        Tones.RememberPerContact = Settings.RememberTone;
        RebuildAccounts();
        RefreshScripts();
        _noKey = string.IsNullOrWhiteSpace(ApiKey);
        _aiState = _noKey ? "还没填写钥匙" : Settings.PresetName + " 已连接";
        if (demo) LoadWechatSample();
        else if (AccountsFile.Accounts.Count == 0) _screen = ScreenKind.Wizard;
        UpdateWizardNote();
    }

    public string ApiKey { get; set; } = "";
    public ScreenKind Screen { get => _screen; private set => Set(ref _screen, value); }
    public bool ShowChat => Screen is ScreenKind.Wechat or ScreenKind.Douyin;
    public bool ShowWizard => Screen == ScreenKind.Wizard;
    public bool ShowSettings => Screen == ScreenKind.Settings;
    public bool ShowWechat => Screen == ScreenKind.Wechat;
    public bool ShowDouyin => Screen == ScreenKind.Douyin;
    public bool WechatMissing { get => _wechatMissing; set => Set(ref _wechatMissing, value); }
    public bool PolishMode { get => _polishMode; set { if (Set(ref _polishMode, value)) Raise(nameof(SuggestMode)); } }
    public bool SuggestMode => !PolishMode;
    public bool Expanded { get => _expanded; set => Set(ref _expanded, value); }
    public bool DrawerOpen { get => _drawer; set => Set(ref _drawer, value); }
    public bool PanelOpen { get => _panelOpen; set => Set(ref _panelOpen, value); }
    public bool NoKey { get => _noKey; private set => Set(ref _noKey, value); }
    public string Tone { get => _tone; set => Set(ref _tone, value); }
    public string Goal { get => _goal; set => Set(ref _goal, value); }
    public string Quote { get => _quote; set => Set(ref _quote, value); }
    public string Emotion { get => _emotion; set => Set(ref _emotion, value); }
    public string ContactLine { get => _contactLine; set => Set(ref _contactLine, value); }
    public string Initial { get => _initial; set => Set(ref _initial, value); }
    public string MemoryLine { get => _memoryLine; set => Set(ref _memoryLine, value); }
    public string Status { get => _status; set => Set(ref _status, value); }
    public string TitleAccount { get => _titleAccount; set => Set(ref _titleAccount, value); }
    public string TitleState { get => _titleState; set => Set(ref _titleState, value); }
    public string ExtraChip { get => _extraChip; set => Set(ref _extraChip, value); }
    public string Draft { get => _draft; set => Set(ref _draft, value); }
    public string ScriptCategory { get => _scriptCategory; set { if (Set(ref _scriptCategory, value)) RefreshScripts(); } }
    public string AiState { get => _aiState; private set => Set(ref _aiState, value); }
    public string WizardPlatform { get => _wizardPlatform; set { if (Set(ref _wizardPlatform, value)) UpdateWizardNote(); } }
    public string WizardBrand { get => _wizardBrand; set { if (Set(ref _wizardBrand, value)) UpdateWizardNote(); } }
    public string WizardName { get => _wizardName; set => Set(ref _wizardName, value); }
    public string WizardNote { get => _wizardNote; private set => Set(ref _wizardNote, value); }
    public string WizardWarn { get => _wizardWarn; private set => Set(ref _wizardWarn, value); }
    public string LaunchHint { get => _launchHint; set => Set(ref _launchHint, value); }
    public string EnvLine { get => _envLine; set => Set(ref _envLine, value); }
    public string MoreLabel => _expanded ? "收起" : "还有 " + Math.Max(0, _all.Count - Settings.SuggestionCount) + " 条";
    public bool HasMore => _all.Count > Settings.SuggestionCount;
    public bool ShowReplyTools => _all.Count > 0;
    public bool LiveWechat => !Demo && ShowWechat;
    public string Promise => "只给建议，永远由你自己按发送；没有群发，没有自动回复";

    public event Action<string, bool>? Deliver;
    public event Action? AccountsChanged;
    public event Action<string>? DouyinSelected;
    public event Action? WechatSelected;
    public event Action<string>? Toast;

    public void ApplyShot(string name)
    {
        switch (name)
        {
            case "douyin":
                LoadDouyinSample();
                break;
            case "wizard":
                Screen = ScreenKind.Wizard;
                WizardPlatform = "douyin";
                WizardBrand = "Auto";
                WizardName = "店铺号";
                TitleAccount = "添加账号";
                TitleState = "首次使用引导";
                ExtraChip = "";
                MarkSelected("add");
                UpdateWizardNote();
                break;
            case "settings":
                Screen = ScreenKind.Settings;
                TitleAccount = "设置 · 话术";
                TitleState = "自动保存";
                ExtraChip = "";
                ScriptCategory = "拒绝";
                MarkSelected("");
                break;
            case "replies":
                LoadWechatSample();
                Expanded = true;
                DrawerOpen = true;
                RebuildVisible();
                break;
            default:
                LoadWechatSample();
                break;
        }
        RaiseChrome();
    }

    public void Select(string id)
    {
        if (id == "add")
        {
            OpenWizard();
            return;
        }
        foreach (var account in Accounts) account.Selected = account.Id == id;
        var chosen = AccountsFile.Accounts.FirstOrDefault(a => a.Id == id);
        if (chosen is null) return;
        if (chosen.Platform == "wechat")
        {
            Screen = ScreenKind.Wechat;
            _platform = "微信";
            TitleAccount = "微信 · " + chosen.Sub;
            if (Demo) LoadWechatSample();
            WechatSelected?.Invoke();
        }
        else
        {
            Screen = ScreenKind.Douyin;
            _platform = "抖音";
            _contact = chosen.Name;
            TitleAccount = chosen.Name + " · " + chosen.Sub;
            TitleState = "独立环境 · 不共享登录";
            var brand = BrowserBrands.Label(BrowserBrands.Parse(chosen.Brand));
            EnvLine = "独立环境 · " + chosen.Name + "     数据 单独文件夹     浏览器 " + brand + "（固定）     时区 北京时间     网络 国内直连 · 不走 VPN";
            if (Demo && chosen.Id == "dy1") LoadDouyinSample();
            DouyinSelected?.Invoke(chosen.Id);
            RememberTone();
        }
        RaiseChrome();
        AccountsChanged?.Invoke();
    }

    public void OpenWizard()
    {
        Screen = ScreenKind.Wizard;
        TitleAccount = "添加账号";
        TitleState = AccountsFile.FirstRunDone ? "添加账号" : "首次使用引导";
        foreach (var account in Accounts) account.Selected = account.IsAdd;
        UpdateWizardNote();
        RaiseChrome();
    }

    public void OpenSettings()
    {
        Screen = ScreenKind.Settings;
        TitleAccount = "设置 · 话术";
        TitleState = "自动保存";
        foreach (var account in Accounts) account.Selected = false;
        RaiseChrome();
    }

    public void CloseOverlay()
    {
        var first = AccountsFile.Accounts.FirstOrDefault();
        if (first is null) OpenWizard();
        else Select(first.Id);
    }

    public void SetTone(string tone)
    {
        Tone = ToneMemory.Normalize(tone);
        Tones.Set(_platform, _contact, Tone);
        MemoryLine = "已记住：跟" + _contact + "默认用「" + Tone + "」";
        if (_persist) LocalStore.SaveTones(Tones);
    }

    public void ToggleExpand()
    {
        Expanded = !Expanded;
        RebuildVisible();
    }

    public void ToggleDrawer() => DrawerOpen = !DrawerOpen;

    public void UseScript(string id)
    {
        var item = Scripts.Items.FirstOrDefault(s => s.Id == id);
        if (item is null) return;
        var points = string.Join("，", item.Points);
        Goal = string.IsNullOrWhiteSpace(Goal) ? points : Goal + "；" + points;
        item.Used++;
        item.LastUsed = "刚刚";
        if (_persist) LocalStore.SaveScripts(Scripts);
        RefreshScripts();
        DrawerOpen = false;
        Toast?.Invoke("已把要点放进「我想表达」");
    }

    public void AddScript(string category, string title, string triggers, string points, string avoid)
    {
        if (string.IsNullOrWhiteSpace(title)) return;
        Scripts.Items.Add(new ScriptItem
        {
            Category = ScriptCategories.All.Contains(category) ? category : "问候",
            Title = title.Trim(),
            Triggers = triggers.Trim(),
            Points = points.Split(['\n', '|', '，'], StringSplitOptions.RemoveEmptyEntries).Select(p => p.Trim()).Where(p => p.Length > 0).ToList(),
            Avoid = avoid.Trim()
        });
        ScriptCategory = category;
        if (_persist) LocalStore.SaveScripts(Scripts);
        RefreshScripts();
    }

    public void ImportCsv(string csv)
    {
        var incoming = ScriptLibrary.FromCsv(csv);
        Scripts.Items.AddRange(incoming.Items);
        if (_persist) LocalStore.SaveScripts(Scripts);
        RefreshScripts();
        Toast?.Invoke("已导入 " + incoming.Items.Count + " 条话术");
    }

    public string ExportCsv() => Scripts.ToCsv();

    public void SaveSettingsFromUi()
    {
        Settings.DefaultTone = ToneMemory.Normalize(Settings.DefaultTone);
        Settings.SuggestionCount = Math.Clamp(Settings.SuggestionCount, 3, 5);
        Settings.SleepMinutes = Math.Clamp(Settings.SleepMinutes, 5, 60);
        Settings.ApiKeyProtected = string.IsNullOrWhiteSpace(ApiKey) ? "" : SecretBox.Protect(ApiKey.Trim());
        NoKey = string.IsNullOrWhiteSpace(ApiKey);
        AiState = NoKey ? "还没填写钥匙" : Settings.PresetName + " 已连接";
        Tones.DefaultTone = Settings.DefaultTone;
        Tones.RememberPerContact = Settings.RememberTone;
        if (_persist)
        {
            LocalStore.SaveSettings(Settings);
            LocalStore.SaveTones(Tones);
        }
    }

    public void ApplyPreset(string name)
    {
        var preset = AiPreset.All.FirstOrDefault(p => p.Name == name) ?? AiPreset.DeepSeek;
        Settings.PresetName = preset.Name;
        Settings.BaseUrl = preset.BaseUrl;
        Settings.Model = preset.Model;
        Raise(nameof(Settings));
        SaveSettingsFromUi();
    }

    public AccountRecord? FinishWizard()
    {
        var name = string.IsNullOrWhiteSpace(WizardName) ? (WizardPlatform == "wechat" ? "微信" : "抖音") : WizardName.Trim();
        if (WizardPlatform == "wechat")
        {
            var existing = AccountsFile.Accounts.FirstOrDefault(a => a.Platform == "wechat");
            if (existing is null)
            {
                existing = new AccountRecord { Id = "wechat", Platform = "wechat", Name = "微信", Sub = name };
                AccountsFile.Accounts.Insert(0, existing);
            }
            else existing.Sub = name;
            AccountsFile.FirstRunDone = true;
            PersistAccounts();
            Select(existing.Id);
            return existing;
        }

        var used = AccountsFile.Accounts.Where(a => a.Platform == "douyin").Select(a => BrowserBrands.Parse(a.Brand));
        var picked = BrowserBrands.Parse(WizardBrand);
        if (picked == BrowserBrand.Auto) picked = IdentityAssigner.PickUnused(used, name.GetHashCode());
        var n = AccountsFile.Accounts.Count(a => a.Platform == "douyin") + 1;
        var record = new AccountRecord
        {
            Id = "dy" + DateTime.Now.ToString("HHmmss"),
            Platform = "douyin",
            Name = "抖音 " + n,
            Sub = name,
            Brand = BrowserBrands.Label(picked)
        };
        if (!Demo) LocalStore.SaveIdentity(record.Id, picked);
        AccountsFile.Accounts.Add(record);
        AccountsFile.FirstRunDone = true;
        PersistAccounts();
        Select(record.Id);
        return record;
    }

    public void NoteTheirMessage(string contact, string text, IReadOnlyList<string> history)
    {
        _contact = string.IsNullOrWhiteSpace(contact) ? _contact : contact;
        _lastTheir = text.Trim();
        _history.Clear();
        _history.AddRange(history);
        Quote = text.Trim();
        ContactLine = _contact + " · 刚刚";
        Initial = _contact.Length > 0 ? _contact[..1] : "对";
        Tone = Tones.Get(_platform, _contact);
        MemoryLine = "已记住：跟" + _contact + "默认用「" + Tone + "」";
    }

    public async Task SuggestAsync(SuggestAdjust adjust)
    {
        if (Demo)
        {
            if (adjust == SuggestAdjust.Polish) ShowReplies(PolishSample(), 0);
            else if (adjust == SuggestAdjust.Shorter) ShowReplies(ShortSample(), 0);
            else ShowReplies(WechatSample(), adjust == SuggestAdjust.AnotherBatch ? 1 : 0);
            Status = "演示数据 · 真机接上钥匙后会按对方的话现写";
            return;
        }
        if (string.IsNullOrWhiteSpace(ApiKey))
        {
            NoKey = true;
            Status = "先到设置里填上 AI 钥匙，才能帮你想回复。";
            return;
        }
        Status = "正在想怎么回…";
        var points = Scripts.RelevantPoints(_lastTheir + " " + Goal, null);
        var req = new SuggestRequest
        {
            Relation = "朋友",
            Persona = Settings.Persona,
            Goal = Goal,
            Points = points,
            Tone = Tone,
            Count = adjust == SuggestAdjust.Polish ? 3 : Math.Clamp(Settings.SuggestionCount == 3 ? 5 : Settings.SuggestionCount, 3, 5),
            History = _history.ToList(),
            LastMessage = _lastTheir,
            ContactName = _contact,
            Adjust = adjust,
            AvoidReplies = adjust == SuggestAdjust.AnotherBatch ? _all.Select(r => r.Text).ToList() : [],
            Draft = Draft
        };
        var client = new OpenAiClient(new LlmOptions
        {
            BaseUrl = Settings.BaseUrl,
            ApiKey = ApiKey,
            Model = Settings.Model
        });
        var result = await client.SuggestAsync(req).ConfigureAwait(true);
        if (!result.Ok)
        {
            Status = result.Error ?? "这次没想好，再点一次「换一批」。";
            return;
        }
        Emotion = result.Read.Line;
        ShowReplies(result.Replies, result.Best);
        Status = result.Retried ? "换了一种说法 · 请你自己按发送" : "请你自己按发送";
    }

    public async Task TestAiAsync()
    {
        if (string.IsNullOrWhiteSpace(ApiKey))
        {
            AiState = "还没填写钥匙";
            Toast?.Invoke("先把钥匙粘贴进去。");
            return;
        }
        AiState = "正在测试…";
        try
        {
            var client = new OpenAiClient(new LlmOptions { BaseUrl = Settings.BaseUrl, ApiKey = ApiKey, Model = Settings.Model });
            await client.TestAsync().ConfigureAwait(true);
            AiState = Settings.PresetName + " 已连接";
            NoKey = false;
            SaveSettingsFromUi();
            Toast?.Invoke("通了，可以帮你想回复了。");
        }
        catch (Exception ex)
        {
            AiState = "没连上";
            Toast?.Invoke(ex.Message);
        }
    }

    public void DeliverIndex(int index, bool paste)
    {
        if (index < 0 || index >= _all.Count) return;
        var modePaste = paste && Settings.FillMode != "copy";
        Deliver?.Invoke(_all[index].Text, modePaste);
    }

    public string? TextAt(int index) => index >= 0 && index < _all.Count ? _all[index].Text : null;

    private void ShowReplies(IReadOnlyList<ReplyItem> items, int best)
    {
        _all.Clear();
        _all.AddRange(items);
        if (best < 0 || best >= _all.Count) best = 0;
        if (_all.Count > 0)
        {
            var chosen = _all[best];
            _all.RemoveAt(best);
            _all.Insert(0, chosen);
        }
        Expanded = false;
        RebuildVisible();
        NoKey = false;
    }

    private void RebuildVisible()
    {
        VisibleReplies.Clear();
        var take = Expanded ? _all.Count : Math.Min(Settings.SuggestionCount, _all.Count);
        for (var i = 0; i < take; i++)
        {
            var item = _all[i];
            var best = i == 0;
            VisibleReplies.Add(new ReplyVm
            {
                Index = i,
                Tag = item.Tag,
                Text = item.Text,
                Why = item.Why,
                Best = best,
                Hotkey = "Alt+" + (i + 1),
                TagFg = TagForeground(item.Tag, best),
                TagBg = TagBackground(item.Tag, best)
            });
        }
        Raise(nameof(MoreLabel));
        Raise(nameof(HasMore));
        Raise(nameof(ShowReplyTools));
    }

    private static Brush TagForeground(string tag, bool best)
    {
        if (best) return Freeze(Color.FromRgb(0x2A, 0x1F, 0x0C));
        var color = tag switch
        {
            "温暖体贴" or "温暖" => Color.FromRgb(0xFF, 0xC9, 0xB5),
            "幽默化解" or "幽默" => Color.FromRgb(0xFF, 0xE0, 0x8A),
            "委婉拒绝" or "谦和" or "给台阶" => Color.FromRgb(0xA9, 0xDC, 0xFF),
            "真诚夸赞" => Color.FromRgb(0xF7, 0xE3, 0xB0),
            _ => Color.FromRgb(0xCF, 0xE7, 0xE3)
        };
        return Freeze(color);
    }

    private static Brush TagBackground(string tag, bool best)
    {
        if (best)
        {
            var gold = new LinearGradientBrush(Color.FromRgb(0xF8, 0xE9, 0xC4), Color.FromRgb(0xD2, 0xAE, 0x6E), 90);
            gold.Freeze();
            return gold;
        }
        var tint = tag switch
        {
            "温暖体贴" or "温暖" => Color.FromArgb(0x22, 0xFF, 0x96, 0x78),
            "幽默化解" or "幽默" => Color.FromArgb(0x22, 0xFF, 0xCD, 0x5A),
            _ => Color.FromArgb(0x22, 0x6F, 0xB8, 0xF0)
        };
        return Freeze(tint);
    }

    private static SolidColorBrush Freeze(Color color)
    {
        var brush = new SolidColorBrush(color);
        brush.Freeze();
        return brush;
    }

    private void RefreshScripts()
    {
        VisibleScripts.Clear();
        var n = 1;
        foreach (var item in Scripts.Items.Where(i => i.Category == ScriptCategory))
        {
            VisibleScripts.Add(new ScriptVm
            {
                Id = item.Id,
                Number = n++,
                Title = item.Title,
                Category = item.Category,
                Points = string.Join("   ", item.Points),
                Use = item.Used == 0 ? "还没用过" : "用过 " + item.Used + " 次" + (string.IsNullOrEmpty(item.LastUsed) ? "" : " · " + item.LastUsed)
            });
        }
    }

    private void RebuildAccounts()
    {
        Accounts.Clear();
        foreach (var record in AccountsFile.Accounts)
        {
            Accounts.Add(new AccountVm
            {
                Id = record.Id,
                Platform = record.Platform,
                Title = record.Platform == "wechat" ? "微信" : record.Name,
                Sub = record.Sub,
                Brand = record.Brand,
                IceBadge = record.Platform == "douyin",
                Badge = Demo && record.Id == "wechat" ? "3" : Demo && record.Id == "dy1" ? "12" : Demo && record.Id == "dy3" ? "2" : "",
                Sleeping = Demo && record.Id == "dy2"
            });
        }
        Accounts.Add(new AccountVm { Id = "add", Title = "添加", IsAdd = true, Platform = "add" });
        if (Accounts.Count > 1) Accounts[0].Selected = Screen != ScreenKind.Wizard;
    }

    private void PersistAccounts()
    {
        RebuildAccounts();
        if (_persist) LocalStore.SaveAccounts(AccountsFile);
        AccountsChanged?.Invoke();
    }

    private void RememberTone()
    {
        Tone = Tones.Get(_platform, _contact);
        MemoryLine = "已记住：跟" + _contact + "默认用「" + Tone + "」";
    }

    private void UpdateWizardNote()
    {
        var used = AccountsFile.Accounts.Where(a => a.Platform == "douyin").Select(a => BrowserBrands.Parse(a.Brand));
        var picked = BrowserBrands.Parse(WizardBrand);
        if (picked == BrowserBrand.Auto) picked = IdentityAssigner.PickUnused(used, 1);
        WizardNote = "自动：给这个号挑一个别的号没用过的，这次分到 " + BrowserBrands.Label(picked);
        WizardWarn = BrowserBrands.Warning(BrowserBrands.Parse(WizardBrand));
    }

    private void RaiseChrome()
    {
        Raise(nameof(ShowChat));
        Raise(nameof(ShowWizard));
        Raise(nameof(ShowSettings));
        Raise(nameof(ShowWechat));
        Raise(nameof(ShowDouyin));
        Raise(nameof(LiveWechat));
    }

    private void MarkSelected(string id)
    {
        foreach (var account in Accounts) account.Selected = account.Id == id;
    }

    private void LoadWechatSample()
    {
        Screen = ScreenKind.Wechat;
        _platform = "微信";
        _contact = "阿杰";
        _lastTheir = "你可一定要来啊，就等你了😄";
        _history.Clear();
        _history.Add("对方：兄弟！我新房终于装好了🎉");
        _history.Add("我：恭喜恭喜！折腾大半年终于能住了");
        _history.Add("对方：周六中午来暖房呗，我亲自下厨，叫了老王他们");
        Goal = "陪孩子复查去不了，改约下周";
        Quote = "周六中午来暖房呗，我亲自下厨，叫了老王他们。你可一定要来啊，就等你了😄";
        Emotion = "开心期待 · 很在乎你来";
        ContactLine = "阿杰 · 好朋友 · 刚刚";
        Initial = "杰";
        Tone = "温暖";
        MemoryLine = "已记住：跟阿杰默认用「温暖」";
        TitleAccount = "微信 · 海外号";
        TitleState = Demo ? "演示画面 · 真机上这里是你的微信" : "已放入 · 微信本身没改动";
        ExtraChip = "新消息自动出建议";
        PolishMode = false;
        DrawerOpen = false;
        ShowReplies(WechatSample(), 0);
        Tones.Set("微信", "阿杰", "温暖");
        MarkSelected("wechat");
    }

    private void LoadDouyinSample()
    {
        Screen = ScreenKind.Douyin;
        _platform = "抖音";
        _contact = "林小雨";
        PolishMode = true;
        Draft = "在深圳 瞎混呗 你呢";
        ShowReplies(PolishSample(), 0);
        Emotion = "惊喜热络 · 想叙旧";
        Quote = "天哪是你吗！好久不见！刷到你的视频了，你现在在哪发展呀？";
        ContactLine = "林小雨 · 老同学 · 刚刚";
        Initial = "雨";
        Tone = "谦和";
        MemoryLine = "已记住：跟林小雨默认用「谦和」";
        TitleAccount = "抖音 1 · 主号";
        TitleState = "独立环境 · 不共享登录";
        ExtraChip = "抖音 2 休眠中 · 省 180 MB";
        EnvLine = "独立环境 · 抖音 1     数据 单独文件夹     浏览器 Edge（固定）     时区 北京时间     网络 国内直连 · 不走 VPN";
        MarkSelected("dy1");
        RaiseChrome();
    }

    private static List<ReplyItem> WechatSample() =>
    [
        new() { Tag = "温暖体贴", Text = "太替你高兴了！可惜周六要陪孩子去医院复查，下周我单独上门给你暖房～", Why = "先接住他的喜悦，再给真实原因和补偿" },
        new() { Tag = "幽默化解", Text = "你下厨我必须到场品鉴！可周六得陪娃复查😭 下周我带酒来补作业", Why = "玩笑冲淡拒绝，他不会觉得被冷落" },
        new() { Tag = "委婉拒绝", Text = "这么重要的日子真不想错过，周六得陪孩子复查，替我跟老王他们说声抱歉", Why = "先表重视再说原因，拒绝不伤人" },
        new() { Tag = "真诚夸赞", Text = "看照片就知道装得特别用心！周六实在走不开，下周一定来参观学习", Why = "夸具体的用心，他的辛苦被看见" },
        new() { Tag = "轻松简短", Text = "恭喜乔迁！周六陪娃复查去不了，下周请你吃饭补上", Why = "跟他的轻快语气一致，不拖沓" }
    ];

    private static List<ReplyItem> ShortSample() =>
    [
        new() { Tag = "轻松简短", Text = "恭喜乔迁！周六陪娃复查去不了，下周请你吃饭", Why = "短，但原因和补偿都在" },
        new() { Tag = "温暖体贴", Text = "太为你高兴了，周六得陪娃复查，下周上门暖房", Why = "先高兴，再把事说清" },
        new() { Tag = "委婉拒绝", Text = "这日子真不想错过，周六走不开，替我和老王说声抱歉", Why = "拒绝清楚，也不扫兴" }
    ];

    private static List<ReplyItem> PolishSample() =>
    [
        new() { Tag = "谦和", Text = "好久不见呀！我在深圳这边，还在慢慢打拼。你现在在哪儿发展？", Why = "“瞎混”换成“打拼”，更体面" },
        new() { Tag = "温暖", Text = "真的是我😄 好久不见！我在深圳，你呢？最近都还好吧", Why = "先回应她的惊喜，再关心她" },
        new() { Tag = "幽默", Text = "被你抓到了😂 在深圳搬砖呢，你在哪？有空来深圳我请你吃饭", Why = "自嘲拉近距离，还顺手约了饭" }
    ];

    private static AccountFile DemoAccounts() => new()
    {
        FirstRunDone = true,
        Accounts =
        [
            new AccountRecord { Id = "wechat", Platform = "wechat", Name = "微信", Sub = "海外号" },
            new AccountRecord { Id = "dy1", Platform = "douyin", Name = "抖音 1", Sub = "主号", Brand = "Edge" },
            new AccountRecord { Id = "dy2", Platform = "douyin", Name = "抖音 2", Sub = "休眠中", Brand = "Chrome" },
            new AccountRecord { Id = "dy3", Platform = "douyin", Name = "抖音 3", Sub = "店铺号", Brand = "QQ" }
        ]
    };

    public void SetBadge(string id, int count)
    {
        var account = Accounts.FirstOrDefault(a => a.Id == id);
        if (account is null) return;
        account.Badge = count > 0 ? count.ToString() : "";
    }

    public void SetSleeping(string id, bool sleeping)
    {
        var account = Accounts.FirstOrDefault(a => a.Id == id);
        if (account is null) return;
        account.Sleeping = sleeping;
        var record = AccountsFile.Accounts.FirstOrDefault(a => a.Id == id);
        account.Sub = sleeping ? "休眠中" : record?.Sub ?? account.Sub;
    }
}
