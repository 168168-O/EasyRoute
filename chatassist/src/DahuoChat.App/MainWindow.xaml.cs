using System.ComponentModel;
using System.Windows;
using System.Windows.Interop;
using System.Windows.Media;
using System.Windows.Media.Animation;
using System.Windows.Media.Imaging;
using System.Windows.Threading;

namespace DahuoChat;

public partial class MainWindow : Window
{
    private readonly string? _shotDir;
    private readonly List<int> _hotIds = [];
    private DouyinHost? _douyin;
    private bool _dyWired;
    private bool _inSync;
    private int _embedTries;
    private DateTime _healthAfter = DateTime.UtcNow;
    private IntPtr _hwnd;
    private DispatcherTimer? _toastTimer;
    private DispatcherTimer? _dyDebounce;
    private string _pendingText = "";
    private string _pendingId = "";
    private string _lastSuggested = "";
    private ulong _hash;
    private bool _seenHash;
    private bool _ocrDue;
    private bool _ocrBusy;
    private DateTime _hashAt;

    public Session Session { get; }

    public MainWindow(Session session, string? shotDir)
    {
        Session = session;
        _shotDir = shotDir;
        InitializeComponent();
        DataContext = session;
        FontFamily = AppFont.Pick();
        session.PropertyChanged += OnSessionProperty;
        session.DouyinSelected += id =>
        {
            if (!session.Demo) _ = ShowDouyin(id);
        };
        session.Deliver += (text, paste) => { _ = DeliverAsync(text, paste); };
        session.Toast += ShowToast;
        SettingsPage.EmbedChanged += _ =>
        {
            if (!session.Demo && session.ShowWechat) SyncHost();
        };
        SettingsPage.Saved += () => RegisterHotkeys();
        ApplyVisuals();
        Loaded += OnLoaded;
    }

    protected override void OnSourceInitialized(EventArgs e)
    {
        base.OnSourceInitialized(e);
        _hwnd = new WindowInteropHelper(this).Handle;
        if (PresentationSource.FromVisual(this) is HwndSource source)
            source.AddHook(WndProc);
        if (!Session.Demo) RegisterHotkeys();
    }

    protected override void OnClosing(CancelEventArgs e)
    {
        WeChatEmbedder.RestoreAll();
        UnregisterHotkeys();
        base.OnClosing(e);
    }

    private async void OnLoaded(object sender, RoutedEventArgs e)
    {
        if (_shotDir is not null)
        {
            try
            {
                await CaptureShots().ConfigureAwait(true);
                Environment.ExitCode = 0;
                Application.Current.Shutdown(0);
            }
            catch (Exception ex)
            {
                try { File.WriteAllText(Path.Combine(_shotDir, "error.txt"), ex.ToString()); }
                catch { /* keep the original failure */ }
                Environment.ExitCode = 1;
                Application.Current.Shutdown(1);
            }
            return;
        }
        if (Session.Demo) return;
        StartTimers();
        SyncHost();
    }

    private void OnSessionProperty(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName is nameof(Session.Screen) or nameof(Session.WechatMissing))
            SyncHost();
        if (e.PropertyName == nameof(Session.PanelOpen))
            AnimatePanel(Session.PanelOpen);
    }

    private void SyncHost()
    {
        if (_inSync) return;
        _inSync = true;
        try
        {
            if (Session.Demo)
            {
                ApplyVisuals();
                return;
            }
            if (!Session.ShowWechat)
            {
                WeChatEmbedder.RestoreAll();
                ApplyVisuals();
                return;
            }
            if (WeChatLocator.FindMainWindow() == IntPtr.Zero)
            {
                WeChatEmbedder.RestoreAll();
                Session.WechatMissing = true;
                Session.LaunchHint = WeChatLocator.FindInstallExe() is null
                    ? "这台电脑上没找到微信。请先安装电脑版微信，自己打开后再回来。"
                    : "微信还没打开。点下面的按钮打开，登录后会自动放进来。";
                ApplyVisuals();
                return;
            }
            Session.WechatMissing = false;
            ApplyVisuals();
            _embedTries = 0;
            Dispatcher.BeginInvoke(EmbedNow, DispatcherPriority.Loaded);
        }
        finally
        {
            _inSync = false;
        }
    }

    private void EmbedNow()
    {
        if (Session.Demo || !Session.ShowWechat || Session.WechatMissing) return;
        if (Session.Settings.EmbedMode == "dock")
        {
            WeChatEmbedder.StartDock(Hwnd(), Metrics);
            Session.TitleState = "贴在旁边 · 微信本身没改动";
            return;
        }
        var host = Slot.SafeHandle();
        if (host == IntPtr.Zero)
        {
            if (++_embedTries > 6) return;
            Dispatcher.BeginInvoke(EmbedNow, DispatcherPriority.Loaded);
            return;
        }
        if (WeChatEmbedder.TryEmbed(host, Hwnd()))
        {
            _healthAfter = DateTime.UtcNow.AddSeconds(3);
            Session.TitleState = "已放入 · 微信本身没改动";
        }
        else
            UseDock("微信没放稳，已改成贴在旁边。");
    }

    private void ApplyVisuals()
    {
        var demo = Session.Demo;
        var wechat = Session.ShowWechat;
        var douyin = Session.ShowDouyin;
        var embed = Session.Settings.EmbedMode != "dock";
        WechatDemo.Visibility = demo && wechat ? Visibility.Visible : Visibility.Collapsed;
        DouyinDemo.Visibility = demo && douyin ? Visibility.Visible : Visibility.Collapsed;
        MissingCard.Visibility = !demo && wechat && Session.WechatMissing ? Visibility.Visible : Visibility.Collapsed;
        Slot.Visibility = !demo && wechat && !Session.WechatMissing && embed ? Visibility.Visible : Visibility.Collapsed;
        DouyinLive.Visibility = !demo && douyin ? Visibility.Visible : Visibility.Collapsed;
    }

    private void UseDock(string toast)
    {
        Session.Settings.EmbedMode = "dock";
        if (!Session.Demo) Session.SaveSettingsFromUi();
        WeChatEmbedder.RestoreAll();
        ApplyVisuals();
        WeChatEmbedder.StartDock(Hwnd(), Metrics);
        Session.TitleState = "贴在旁边 · 微信本身没改动";
        ShowToast(toast);
    }

    private async Task ShowDouyin(string id)
    {
        var account = Session.AccountsFile.Accounts.FirstOrDefault(a => a.Id == id);
        if (account is null) return;
        try
        {
            _douyin ??= new DouyinHost(DouyinMount);
            EnsureDouyinEvents();
            await _douyin.ShowAsync(account).ConfigureAwait(true);
            Session.SetSleeping(id, _douyin.IsAsleep(id));
        }
        catch (Exception ex)
        {
            ShowToast(FriendlyWeb(ex.Message));
        }
    }

    private void EnsureDouyinEvents()
    {
        if (_dyWired || _douyin is null) return;
        _dyWired = true;
        _douyin.TheirMessage += OnTheirMessage;
        _douyin.Unread += (id, count) => Session.SetBadge(id, count);
        _douyin.Failed += msg => ShowToast(FriendlyWeb(msg));
    }

    private void OnTheirMessage(string id, string text)
    {
        _pendingId = id;
        _pendingText = text;
        _dyDebounce ??= new DispatcherTimer { Interval = TimeSpan.FromSeconds(1) };
        _dyDebounce.Stop();
        _dyDebounce.Tick -= OnDouyinDebounced;
        _dyDebounce.Tick += OnDouyinDebounced;
        _dyDebounce.Start();
    }

    private async void OnDouyinDebounced(object? sender, EventArgs e)
    {
        _dyDebounce?.Stop();
        var text = _pendingText.Trim();
        if (text.Length == 0 || text == _lastSuggested) return;
        _lastSuggested = text;
        var account = Session.AccountsFile.Accounts.FirstOrDefault(a => a.Id == _pendingId);
        var name = string.IsNullOrWhiteSpace(account?.Sub) ? account?.Name ?? "对方" : account!.Sub;
        Session.NoteTheirMessage(name, text, ["对方：" + text]);
        if (Session.Settings.AutoSuggest)
            await Session.SuggestAsync(SuggestAdjust.None).ConfigureAwait(true);
    }

    private void StartTimers()
    {
        var ocr = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(1500) };
        ocr.Tick += OnOcrTick;
        ocr.Start();
        var health = new DispatcherTimer { Interval = TimeSpan.FromSeconds(1) };
        health.Tick += (_, _) =>
        {
            if (DateTime.UtcNow < _healthAfter) return;
            if (Session.Settings.EmbedMode == "embed" && WeChatEmbedder.IsEmbedded && WeChatEmbedder.Unhealthy())
                UseDock("微信放进来不太稳，已改成贴在旁边。");
        };
        health.Start();
        var sleep = new DispatcherTimer { Interval = TimeSpan.FromSeconds(30) };
        sleep.Tick += async (_, _) =>
        {
            if (_douyin is null) return;
            try
            {
                await _douyin.MaintainAsync().ConfigureAwait(true);
                foreach (var id in _douyin.AsleepIds) Session.SetSleeping(id, true);
                foreach (var id in _douyin.AwakeIds) Session.SetSleeping(id, false);
            }
            catch (Exception ex)
            {
                Session.Status = FriendlyWeb(ex.Message);
            }
        };
        sleep.Start();
        SizeChanged += (_, _) =>
        {
            if (Session.Settings.EmbedMode == "dock") return;
            WeChatEmbedder.Fit(Slot.SafeHandle());
        };
    }

    private async void OnOcrTick(object? sender, EventArgs e)
    {
        if (Session.Demo || !Session.ShowWechat || Session.WechatMissing) return;
        var frame = WeChatEmbedder.CaptureChat();
        if (frame is null) return;
        var hash = ThumbnailHash.Average(frame.Bgra, frame.Width, frame.Height, frame.Stride);
        if (!_seenHash)
        {
            _hash = hash;
            _seenHash = true;
            return;
        }
        if (ThumbnailHash.Changed(_hash, hash))
        {
            _hash = hash;
            _hashAt = DateTime.UtcNow;
            _ocrDue = true;
            return;
        }
        if (!_ocrDue || _ocrBusy || DateTime.UtcNow - _hashAt < TimeSpan.FromSeconds(1)) return;
        _ocrDue = false;
        _ocrBusy = true;
        try
        {
            var lines = await ChatOcr.ReadAsync(frame).ConfigureAwait(true);
            if (!ChatParser.ShouldSuggest(lines)) return;
            var last = lines[^1];
            if (last.Text == _lastSuggested) return;
            _lastSuggested = last.Text;
            var history = lines.TakeLast(8).Select(line => (line.Who == Speaker.Me ? "我：" : "对方：") + line.Text).ToList();
            Session.NoteTheirMessage("对方", last.Text, history);
            if (Session.Settings.AutoSuggest)
                await Session.SuggestAsync(SuggestAdjust.None).ConfigureAwait(true);
        }
        catch (Exception ex)
        {
            Session.Status = "这次没读出来，过一会儿再看。" + ex.Message;
        }
        finally
        {
            _ocrBusy = false;
        }
    }

    private void ManualRead()
    {
        if (Session.ShowWechat && !Session.Demo)
        {
            _seenHash = true;
            _ocrDue = true;
            _hashAt = DateTime.UtcNow.AddSeconds(-2);
            OnOcrTick(null, EventArgs.Empty);
            return;
        }
        _ = Session.SuggestAsync(Session.PolishMode ? SuggestAdjust.Polish : SuggestAdjust.None);
    }

    private async Task DeliverAsync(string text, bool paste)
    {
        try
        {
            PasteService.SetClipboard(text);
        }
        catch
        {
            ShowToast("没复制成功，请再点一次。");
            return;
        }
        if (!paste || Session.Demo)
        {
            ShowToast(Session.Demo && paste
                ? "已复制。演示画面不会填进聊天框，发送请你自己按。"
                : "已复制。去聊天框粘贴，发送请你自己按。");
            return;
        }
        if (Session.ShowDouyin && _douyin is not null)
        {
            await _douyin.FocusComposerAsync().ConfigureAwait(true);
            PasteService.CtrlV();
            ShowToast("已填入。发送请你自己按。");
            return;
        }
        var hwnd = WeChatEmbedder.Hwnd;
        if (hwnd == IntPtr.Zero) hwnd = WeChatLocator.FindMainWindow();
        if (hwnd != IntPtr.Zero && PasteService.FocusComposer(hwnd))
        {
            await Task.Delay(80).ConfigureAwait(true);
            PasteService.CtrlV();
            ShowToast("已填入输入框。发送请你自己按。");
            return;
        }
        ShowToast("已复制。去聊天框粘贴，发送请你自己按。");
    }

    private void RegisterHotkeys()
    {
        UnregisterHotkeys();
        if (_hwnd == IntPtr.Zero || Session.Demo) return;
        for (var i = 1; i <= 5; i++)
        {
            var id = 10 + i;
            if (Native.RegisterHotKey(_hwnd, id, 0x0001 | 0x4000, (uint)('0' + i)))
                _hotIds.Add(id);
        }
        if (!HotkeySpec.TryParse(Session.Settings.Hotkey, out var mods, out var key))
        {
            ShowToast("快捷键没看懂。请写成 Ctrl+Alt+R 这种。");
            return;
        }
        for (var i = 1; i <= 5; i++)
        {
            if (!HotkeySpec.Conflicts(Session.Settings.Hotkey, "Alt+" + i)) continue;
            ShowToast("这个快捷键和 Alt+" + i + " 撞了，请换一个。");
            return;
        }
        if (Native.RegisterHotKey(_hwnd, 20, mods | 0x4000, key))
            _hotIds.Add(20);
        else
            ShowToast("快捷键被别的软件占用了，请到设置里换一个。");
    }

    private void UnregisterHotkeys()
    {
        if (_hwnd == IntPtr.Zero) return;
        foreach (var id in _hotIds)
            Native.UnregisterHotKey(_hwnd, id);
        _hotIds.Clear();
    }

    private IntPtr WndProc(IntPtr hwnd, int msg, IntPtr wParam, IntPtr lParam, ref bool handled)
    {
        if (msg != Native.WmHotkey) return IntPtr.Zero;
        var id = wParam.ToInt32();
        if (id == 20) ManualRead();
        else if (id is >= 11 and <= 15) Session.DeliverIndex(id - 11, true);
        handled = true;
        return IntPtr.Zero;
    }

    private void AnimatePanel(bool open)
    {
        var animation = new DoubleAnimation(open ? 360 : 28, TimeSpan.FromMilliseconds(180));
        PanelShell.BeginAnimation(FrameworkElement.WidthProperty, animation);
    }

    private (int rail, int panel, int title) Metrics()
    {
        var dpi = VisualTreeHelper.GetDpi(this);
        int Px(double dip) => (int)Math.Round(dip * dpi.DpiScaleX);
        return (Px(RailCol.ActualWidth), Px(PanelShell.ActualWidth), Px(TitleBar.ActualHeight));
    }

    private IntPtr Hwnd() => _hwnd != IntPtr.Zero ? _hwnd : new WindowInteropHelper(this).Handle;

    private async void Launch_Click(object sender, RoutedEventArgs e)
    {
        if (!WeChatLocator.Launch())
        {
            ShowToast("没找到电脑版微信。请先安装，自己打开后再回来。");
            return;
        }
        ShowToast("正在打开微信。登录完成后会自动放进来。");
        for (var i = 0; i < 15; i++)
        {
            await Task.Delay(2000).ConfigureAwait(true);
            if (WeChatLocator.FindMainWindow() == IntPtr.Zero || !Session.ShowWechat) continue;
            SyncHost();
            return;
        }
    }

    private void BadView_Click(object sender, RoutedEventArgs e) => UseDock("已改成贴在旁边。微信本身没有改动。");

    private void Account_Click(object sender, RoutedEventArgs e)
    {
        if (sender is FrameworkElement fe && fe.Tag is string id)
            Session.Select(id);
    }

    private void Settings_Click(object sender, RoutedEventArgs e) => Session.OpenSettings();
    private void Reopen_Click(object sender, RoutedEventArgs e) => Session.PanelOpen = true;
    private void Min_Click(object sender, RoutedEventArgs e) => WindowState = WindowState.Minimized;
    private void Max_Click(object sender, RoutedEventArgs e) => WindowState = WindowState == WindowState.Maximized ? WindowState.Normal : WindowState.Maximized;
    private void Close_Click(object sender, RoutedEventArgs e) => Close();

    private void ShowToast(string text)
    {
        ToastText.Text = text;
        ToastBar.Visibility = Visibility.Visible;
        _toastTimer ??= new DispatcherTimer { Interval = TimeSpan.FromSeconds(2.6) };
        _toastTimer.Stop();
        _toastTimer.Tick -= HideToast;
        _toastTimer.Tick += HideToast;
        _toastTimer.Start();
    }

    private void HideToast(object? sender, EventArgs e)
    {
        _toastTimer?.Stop();
        ToastBar.Visibility = Visibility.Collapsed;
    }

    private async Task CaptureShots()
    {
        Directory.CreateDirectory(_shotDir!);
        WindowState = WindowState.Normal;
        Width = 1366;
        Height = 768;
        Left = 0;
        Top = 0;
        var shots = new (string File, string Name)[]
        {
            ("01-wechat.png", "wechat"),
            ("02-douyin.png", "douyin"),
            ("03-wizard.png", "wizard"),
            ("04-settings.png", "settings"),
            ("05-replies.png", "replies")
        };
        foreach (var (file, name) in shots)
        {
            Session.ApplyShot(name);
            PanelShell.BeginAnimation(FrameworkElement.WidthProperty, null);
            PanelShell.Width = 360;
            await Dispatcher.InvokeAsync(() => { }, DispatcherPriority.ApplicationIdle);
            UpdateLayout();
            var w = Math.Max(1, (int)Math.Round(ActualWidth));
            var h = Math.Max(1, (int)Math.Round(ActualHeight));
            var bmp = new RenderTargetBitmap(w, h, 96, 96, PixelFormats.Pbgra32);
            bmp.Render(this);
            var encoder = new PngBitmapEncoder();
            encoder.Frames.Add(BitmapFrame.Create(bmp));
            var path = Path.Combine(_shotDir!, file);
            using (var stream = File.Create(path))
                encoder.Save(stream);
            if (new FileInfo(path).Length < 8000)
                throw new InvalidOperationException("画面太小：" + file);
        }
    }

    private static string FriendlyWeb(string message)
    {
        if (message.Contains("WebView2", StringComparison.OrdinalIgnoreCase))
            return "还没装网页组件。请先安装微软网页运行环境，然后重新打开。";
        return "这个抖音页没打开。可以关掉再试一次。";
    }
}
