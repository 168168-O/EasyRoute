using System.Windows.Controls;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.Wpf;

namespace DahuoChat;

public sealed class DouyinTab
{
    public string Id { get; init; } = "";
    public WebView2 View { get; init; } = null!;
    public bool Awake { get; set; }
    public DateTimeOffset LastActive { get; set; } = DateTimeOffset.Now;
    public DateTimeOffset? LastPeek { get; set; }
    public bool Ready { get; set; }
}

public sealed class DouyinHost
{
    private readonly Grid _mount;
    private readonly Dictionary<string, DouyinTab> _tabs = new();
    private readonly HashSet<string> _awake = new();
    public event Action<string, string>? TheirMessage;
    public event Action<string, int>? Unread;
    public event Action<string>? Failed;
    public string? ActiveId { get; private set; }

    public DouyinHost(Grid mount) => _mount = mount;

    public async Task ShowAsync(AccountRecord account)
    {
        ActiveId = account.Id;
        foreach (var tab in _tabs.Values)
            tab.View.Visibility = tab.Id == account.Id ? System.Windows.Visibility.Visible : System.Windows.Visibility.Collapsed;
        if (!_tabs.TryGetValue(account.Id, out var existing))
        {
            existing = await CreateAsync(account).ConfigureAwait(true);
            if (existing is null) return;
        }
        existing.LastActive = DateTimeOffset.Now;
        if (!existing.Awake && existing.View.CoreWebView2 is not null)
            existing.View.CoreWebView2.Resume();
        existing.Awake = true;
        _awake.Add(account.Id);
        existing.View.Visibility = System.Windows.Visibility.Visible;
        TrimAwake(account.Id);
    }

    public void Touch(string id)
    {
        if (_tabs.TryGetValue(id, out var tab)) tab.LastActive = DateTimeOffset.Now;
    }

    public async Task MaintainAsync()
    {
        var snaps = _tabs.Values.Select(t => new TabSnapshot(t.Id, _awake.Contains(t.Id), t.LastActive, t.LastPeek)).ToList();
        foreach (var action in SleepPolicy.Tick(snaps, DateTimeOffset.Now))
        {
            if (!_tabs.TryGetValue(action.Id, out var tab) || tab.View.CoreWebView2 is null) continue;
            if (action.Kind == SleepActionKind.Suspend)
                await Sleep(tab).ConfigureAwait(true);
            else if (action.Kind == SleepActionKind.Peek)
                await Peek(tab).ConfigureAwait(true);
        }
    }

    public async Task FocusComposerAsync()
    {
        if (ActiveId is null || !_tabs.TryGetValue(ActiveId, out var tab) || tab.View.CoreWebView2 is null) return;
        tab.View.Focus();
        await tab.View.CoreWebView2.ExecuteScriptAsync(EmbeddedJs.Focus()).ConfigureAwait(true);
    }

    private async Task<DouyinTab?> CreateAsync(AccountRecord account)
    {
        try
        {
            var dir = AppPaths.DouyinRoot(account.Id);
            Directory.CreateDirectory(dir);
            var brand = LocalStore.LoadBrand(account.Id, account.Brand);
            LocalStore.SaveIdentity(account.Id, brand);
            var options = new CoreWebView2EnvironmentOptions
            {
                Language = "zh-CN",
                AdditionalBrowserArguments = "--lang=zh-CN --disable-features=msSmartScreenProtection"
            };
            var env = await CoreWebView2Environment.CreateAsync(null, dir, options).ConfigureAwait(true);
            var view = new WebView2();
            _mount.Children.Add(view);
            await view.EnsureCoreWebView2Async(env).ConfigureAwait(true);
            var core = view.CoreWebView2;
            var profile = UserAgentFactory.Create(brand, env.BrowserVersionString);
            await core.CallDevToolsProtocolMethodAsync("Emulation.setUserAgentOverride", UserAgentFactory.ToCdpJson(profile)).ConfigureAwait(true);
            if (!IsBeijingTime())
                await core.CallDevToolsProtocolMethodAsync("Emulation.setTimezoneOverride", "{\"timezoneId\":\"Asia/Shanghai\"}").ConfigureAwait(true);
            core.Settings.AreDevToolsEnabled = false;
            core.Settings.IsStatusBarEnabled = false;
            core.Settings.AreBrowserAcceleratorKeysEnabled = false;
            core.NavigationStarting += (_, e) =>
            {
                if (!DouyinHosts.IsAllowed(e.Uri)) e.Cancel = true;
            };
            await core.AddScriptToExecuteOnDocumentCreatedAsync(EmbeddedJs.Observer()).ConfigureAwait(true);
            var tab = new DouyinTab { Id = account.Id, View = view, Awake = true, Ready = true };
            core.WebMessageReceived += (_, e) => OnMessage(account.Id, e.TryGetWebMessageAsString());
            _tabs[account.Id] = tab;
            core.Navigate(DouyinHosts.Home);
            return tab;
        }
        catch (Exception ex)
        {
            Failed?.Invoke(ex.Message);
            return null;
        }
    }

    private void OnMessage(string id, string json)
    {
        if (!ObserverMessages.TryParse(json, out var who, out var text, out var unread)) return;
        if (unread is int n)
        {
            Unread?.Invoke(id, n);
            return;
        }
        if (who == "them" && text.Length > 0)
            TheirMessage?.Invoke(id, text);
    }

    private void TrimAwake(string keep)
    {
        while (_awake.Count > SleepPolicy.MaxAwake)
        {
            var victim = _tabs.Values
                .Where(t => _awake.Contains(t.Id) && t.Id != keep)
                .OrderBy(t => t.LastActive)
                .FirstOrDefault();
            if (victim is null) break;
            _ = Sleep(victim);
        }
    }

    private async Task Sleep(DouyinTab tab)
    {
        _awake.Remove(tab.Id);
        tab.Awake = false;
        if (tab.View.CoreWebView2 is null) return;
        tab.View.CoreWebView2.MemoryUsageTargetLevel = CoreWebView2MemoryUsageTargetLevel.Low;
        await tab.View.CoreWebView2.TrySuspendAsync().ConfigureAwait(true);
    }

    private async Task Peek(DouyinTab tab)
    {
        if (tab.View.CoreWebView2 is null) return;
        tab.LastPeek = DateTimeOffset.Now;
        tab.View.CoreWebView2.Resume();
        await Task.Delay(1500).ConfigureAwait(true);
        if (ActiveId != tab.Id)
            await Sleep(tab).ConfigureAwait(true);
    }

    private static bool IsBeijingTime()
    {
        var id = TimeZoneInfo.Local.Id;
        return id is "China Standard Time" or "Asia/Shanghai" or "中国标准时间";
    }

    public bool IsAsleep(string id) => _tabs.ContainsKey(id) && !_awake.Contains(id);
    public IEnumerable<string> AsleepIds => _tabs.Keys.Where(id => !_awake.Contains(id));
    public IEnumerable<string> AwakeIds => _awake.ToArray();
}
