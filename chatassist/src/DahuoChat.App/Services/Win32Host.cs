using System.Runtime.InteropServices;
using System.Text;

namespace DahuoChat;

public static class Native
{
    public const int GwlStyle = -16;
    public const int WsCaption = 0x00C00000;
    public const int WsThickFrame = 0x00040000;
    public const int WsBorder = 0x00800000;
    public const int WsDlgFrame = 0x00400000;
    public const int WsSysMenu = 0x00080000;
    public const int WsMinimizeBox = 0x00020000;
    public const int WsMaximizeBox = 0x00010000;
    public const int WsChild = 0x40000000;
    public const int WsPopup = unchecked((int)0x80000000);
    public const int WsVisible = 0x10000000;
    public const uint SwpFrameChanged = 0x0020;
    public const uint SwpNoZOrder = 0x0004;
    public const uint SwpNoActivate = 0x0010;
    public const uint SwpShowWindow = 0x0040;
    public const uint EventObjectLocationChange = 0x800B;
    public const uint WineventOutOfContext = 0x0000;
    public const uint WineventSkipOwnProcess = 0x0002;
    public const uint SrcCopy = 0x00CC0020;
    public const int WmHotkey = 0x0312;
    public const uint InputKeyboard = 1;
    public const uint InputMouse = 0;
    public const uint KeyeventfKeyup = 0x0002;
    public const uint MouseeventfLeftDown = 0x0002;
    public const uint MouseeventfLeftUp = 0x0004;
    public const uint MouseeventfAbsolute = 0x8000;
    public const uint MouseeventfMove = 0x0001;
    public const int SwRestore = 9;
    public const int SmCxScreen = 0;
    public const int SmCyScreen = 1;

    public delegate bool EnumWindowsProc(IntPtr hwnd, IntPtr lParam);
    public delegate void WinEventProc(IntPtr hook, uint eventType, IntPtr hwnd, int idObject, int idChild, uint thread, uint time);

    [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
    [DllImport("user32.dll")] public static extern bool IsWindow(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr hwnd, StringBuilder lpString, int nMaxCount);
    [DllImport("user32.dll")] public static extern int GetWindowTextLength(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint processId);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd, out Rect lpRect);
    [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr hwnd, out Rect lpRect);
    [DllImport("user32.dll")] public static extern IntPtr GetParent(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern IntPtr SetParent(IntPtr child, IntPtr newParent);
    [DllImport("user32.dll", EntryPoint = "GetWindowLongPtrW", SetLastError = true)] public static extern IntPtr GetWindowLongPtr(IntPtr hwnd, int index);
    [DllImport("user32.dll", EntryPoint = "SetWindowLongPtrW", SetLastError = true)] public static extern IntPtr SetWindowLongPtr(IntPtr hwnd, int index, IntPtr newLong);
    [DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr hwnd, int x, int y, int width, int height, bool repaint);
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hwnd, IntPtr after, int x, int y, int cx, int cy, uint flags);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hwnd, int cmd);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr hwnd, ref Point pt);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern uint SendInput(uint n, INPUT[] inputs, int size);
    [DllImport("user32.dll")] public static extern bool RegisterHotKey(IntPtr hwnd, int id, uint fsModifiers, uint vk);
    [DllImport("user32.dll")] public static extern bool UnregisterHotKey(IntPtr hwnd, int id);
    [DllImport("user32.dll")] public static extern IntPtr SetWinEventHook(uint min, uint max, IntPtr module, WinEventProc proc, uint pid, uint thread, uint flags);
    [DllImport("user32.dll")] public static extern bool UnhookWinEvent(IntPtr hook);
    [DllImport("user32.dll")] public static extern IntPtr GetDC(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern int ReleaseDC(IntPtr hwnd, IntPtr hdc);
    [DllImport("gdi32.dll")] public static extern IntPtr CreateCompatibleDC(IntPtr hdc);
    [DllImport("gdi32.dll")] public static extern IntPtr CreateCompatibleBitmap(IntPtr hdc, int w, int h);
    [DllImport("gdi32.dll")] public static extern IntPtr SelectObject(IntPtr hdc, IntPtr obj);
    [DllImport("gdi32.dll")] public static extern bool DeleteObject(IntPtr obj);
    [DllImport("gdi32.dll")] public static extern bool DeleteDC(IntPtr hdc);
    [DllImport("gdi32.dll")] public static extern bool BitBlt(IntPtr dst, int x, int y, int w, int h, IntPtr src, int sx, int sy, uint rop);
    [DllImport("gdi32.dll")] public static extern int GetDIBits(IntPtr hdc, IntPtr hbmp, uint start, uint lines, byte[] bits, ref BITMAPINFO bmi, uint usage);
    [DllImport("user32.dll")] public static extern int GetSystemMetrics(int index);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr CreateWindowEx(int ex, string cls, string name, int style, int x, int y, int w, int h, IntPtr parent, IntPtr menu, IntPtr instance, IntPtr param);
    [DllImport("user32.dll")] public static extern bool DestroyWindow(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern IntPtr SetWindowRgn(IntPtr hwnd, IntPtr rgn, bool redraw);
    [DllImport("gdi32.dll")] public static extern IntPtr CreateRectRgn(int l, int t, int r, int b);
    [DllImport("gdi32.dll")] public static extern int CombineRgn(IntPtr dest, IntPtr src1, IntPtr src2, int mode);
    [DllImport("kernel32.dll")] public static extern IntPtr GetModuleHandle(string? name);

    [StructLayout(LayoutKind.Sequential)]
    public struct Rect
    {
        public int Left, Top, Right, Bottom;
        public int Width => Right - Left;
        public int Height => Bottom - Top;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct Point
    {
        public int X;
        public int Y;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct INPUT
    {
        public uint type;
        public InputUnion U;
    }

    [StructLayout(LayoutKind.Explicit)]
    public struct InputUnion
    {
        [FieldOffset(0)] public MOUSEINPUT mi;
        [FieldOffset(0)] public KEYBDINPUT ki;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct MOUSEINPUT
    {
        public int dx;
        public int dy;
        public uint mouseData;
        public uint dwFlags;
        public uint time;
        public IntPtr dwExtraInfo;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct KEYBDINPUT
    {
        public ushort wVk;
        public ushort wScan;
        public uint dwFlags;
        public uint time;
        public IntPtr dwExtraInfo;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct BITMAPINFOHEADER
    {
        public uint biSize;
        public int biWidth;
        public int biHeight;
        public ushort biPlanes;
        public ushort biBitCount;
        public uint biCompression;
        public uint biSizeImage;
        public int biXPelsPerMeter;
        public int biYPelsPerMeter;
        public uint biClrUsed;
        public uint biClrImportant;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct BITMAPINFO
    {
        public BITMAPINFOHEADER bmiHeader;
        public uint bmiColors;
    }

    public static string Title(IntPtr hwnd)
    {
        var len = GetWindowTextLength(hwnd);
        if (len <= 0) return "";
        var sb = new StringBuilder(len + 2);
        GetWindowText(hwnd, sb, sb.Capacity);
        return sb.ToString();
    }
}

public sealed class EmbedSnapshot
{
    public IntPtr Hwnd;
    public IntPtr Parent;
    public long Style;
    public Native.Rect Rect;
}

public static class WeChatLocator
{
    public static IntPtr FindMainWindow()
    {
        IntPtr best = IntPtr.Zero;
        var bestArea = 0;
        var self = Environment.ProcessId;
        Native.EnumWindows((hwnd, _) =>
        {
            if (!Native.IsWindowVisible(hwnd)) return true;
            if (Native.GetParent(hwnd) != IntPtr.Zero) return true;
            Native.GetWindowThreadProcessId(hwnd, out var pid);
            if (pid == 0 || pid == self) return true;
            string name;
            try
            {
                using var proc = System.Diagnostics.Process.GetProcessById((int)pid);
                name = proc.ProcessName;
            }
            catch
            {
                return true;
            }
            if (!name.Equals("Weixin", StringComparison.OrdinalIgnoreCase) &&
                !name.Equals("WeChat", StringComparison.OrdinalIgnoreCase))
                return true;
            if (!Native.GetWindowRect(hwnd, out var rc)) return true;
            var area = rc.Width * rc.Height;
            if (area < 200 * 200) return true;
            var title = Native.Title(hwnd);
            if (title.Length == 0) return true;
            if (area > bestArea)
            {
                bestArea = area;
                best = hwnd;
            }
            return true;
        }, IntPtr.Zero);
        return best;
    }

    public static string? FindInstallExe()
    {
        var regs = new List<string?>();
        foreach (var path in new[] { @"Software\Tencent\Weixin", @"Software\Tencent\WeChat" })
        {
            try
            {
                using var key = Microsoft.Win32.Registry.CurrentUser.OpenSubKey(path);
                regs.Add(key?.GetValue("InstallPath") as string);
            }
            catch { regs.Add(null); }
        }
        foreach (var path in new[] { @"SOFTWARE\WOW6432Node\Tencent\Weixin", @"SOFTWARE\WOW6432Node\Tencent\WeChat", @"SOFTWARE\Tencent\Weixin", @"SOFTWARE\Tencent\WeChat" })
        {
            try
            {
                using var key = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(path);
                regs.Add(key?.GetValue("InstallPath") as string);
            }
            catch { regs.Add(null); }
        }
        var roots = new[]
        {
            Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles),
            Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86),
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Tencent")
        };
        foreach (var candidate in WeChatPaths.Candidates(regs, roots))
        {
            if (File.Exists(candidate)) return candidate;
        }
        return null;
    }

    public static bool Launch()
    {
        var exe = FindInstallExe();
        if (exe is null) return false;
        System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(exe) { UseShellExecute = true });
        return true;
    }
}

public static class WeChatEmbedder
{
    private static EmbedSnapshot? _snap;
    private static IntPtr _hook;
    private static Native.WinEventProc? _proc;
    private static System.Windows.Threading.DispatcherTimer? _dockTimer;
    private static IntPtr _ourWindow;
    private static Func<(int rail, int panel, int title)>? _metrics;
    private static int _foreignFocus;
    private static DateTime _embedStarted;

    public static bool IsEmbedded => _snap is not null && Native.IsWindow(_snap.Hwnd);
    public static IntPtr Hwnd => _snap?.Hwnd ?? IntPtr.Zero;

    public static bool TryEmbed(IntPtr host, IntPtr ourWindow)
    {
        RestoreAll();
        var hwnd = WeChatLocator.FindMainWindow();
        if (hwnd == IntPtr.Zero || host == IntPtr.Zero) return false;
        Native.ShowWindow(hwnd, Native.SwRestore);
        var snap = new EmbedSnapshot
        {
            Hwnd = hwnd,
            Parent = Native.GetParent(hwnd),
            Style = Native.GetWindowLongPtr(hwnd, Native.GwlStyle).ToInt64()
        };
        Native.GetWindowRect(hwnd, out snap.Rect);
        var style = (int)snap.Style;
        style &= ~(Native.WsCaption | Native.WsThickFrame | Native.WsBorder | Native.WsDlgFrame | Native.WsSysMenu | Native.WsMinimizeBox | Native.WsMaximizeBox | Native.WsPopup);
        style |= Native.WsChild | Native.WsVisible;
        Native.SetWindowLongPtr(hwnd, Native.GwlStyle, new IntPtr(style));
        Native.SetParent(hwnd, host);
        _snap = snap;
        _ourWindow = ourWindow;
        _embedStarted = DateTime.UtcNow;
        _foreignFocus = 0;
        Fit(host);
        return Native.IsWindow(hwnd);
    }

    public static void Fit(IntPtr host)
    {
        if (_snap is null || !Native.IsWindow(_snap.Hwnd) || host == IntPtr.Zero) return;
        if (!Native.GetClientRect(host, out var rc)) return;
        Native.MoveWindow(_snap.Hwnd, 0, 0, Math.Max(rc.Width, 1), Math.Max(rc.Height, 1), true);
    }

    public static bool Unhealthy()
    {
        if (_snap is null) return false;
        var alive = Native.IsWindow(_snap.Hwnd);
        var w = 0;
        var h = 0;
        if (alive && Native.GetWindowRect(_snap.Hwnd, out var rc))
        {
            w = rc.Width;
            h = rc.Height;
        }
        if (alive && (DateTime.UtcNow - _embedStarted).TotalSeconds < 3)
        {
            var fg = Native.GetForegroundWindow();
            var ours = fg == _ourWindow || fg == _snap.Hwnd || Native.GetParent(fg) == _snap.Hwnd;
            if (!ours) _foreignFocus++;
            else _foreignFocus = 0;
        }
        return EmbedHealth.ShouldFallback(alive, w, h, _foreignFocus, false);
    }

    public static void RestoreAll()
    {
        StopDock();
        if (_snap is null) return;
        var snap = _snap;
        _snap = null;
        if (!Native.IsWindow(snap.Hwnd)) return;
        Native.SetParent(snap.Hwnd, snap.Parent);
        Native.SetWindowLongPtr(snap.Hwnd, Native.GwlStyle, new IntPtr(snap.Style));
        Native.SetWindowPos(snap.Hwnd, IntPtr.Zero, snap.Rect.Left, snap.Rect.Top, snap.Rect.Width, snap.Rect.Height,
            Native.SwpFrameChanged | Native.SwpNoZOrder | Native.SwpShowWindow);
    }

    public static void StartDock(IntPtr ourWindow, Func<(int rail, int panel, int title)> metrics)
    {
        RestoreAll();
        _ourWindow = ourWindow;
        _metrics = metrics;
        var hwnd = WeChatLocator.FindMainWindow();
        if (hwnd == IntPtr.Zero) return;
        _snap = new EmbedSnapshot { Hwnd = hwnd };
        _proc = OnEvent;
        Native.GetWindowThreadProcessId(hwnd, out var pid);
        _hook = Native.SetWinEventHook(Native.EventObjectLocationChange, Native.EventObjectLocationChange, IntPtr.Zero, _proc, pid, 0,
            Native.WineventOutOfContext | Native.WineventSkipOwnProcess);
        _dockTimer = new System.Windows.Threading.DispatcherTimer { Interval = TimeSpan.FromMilliseconds(400) };
        _dockTimer.Tick += (_, _) => Follow();
        _dockTimer.Start();
        Follow();
    }

    private static void OnEvent(IntPtr hook, uint eventType, IntPtr hwnd, int idObject, int idChild, uint thread, uint time)
    {
        if (_snap is null || hwnd != _snap.Hwnd) return;
        System.Windows.Application.Current?.Dispatcher.BeginInvoke(Follow);
    }

    public static void Follow()
    {
        if (_snap is null || _metrics is null || !Native.IsWindow(_snap.Hwnd) || _ourWindow == IntPtr.Zero) return;
        if (!Native.GetWindowRect(_snap.Hwnd, out var wr)) return;
        var (rail, panel, title) = _metrics();
        var width = rail + Math.Max(wr.Width, 1) + panel;
        var height = title + Math.Max(wr.Height, 1);
        var left = wr.Left - rail;
        var top = wr.Top - title;
        Native.SetWindowPos(_ourWindow, new IntPtr(-1), left, top, width, height, Native.SwpNoActivate);
        var full = Native.CreateRectRgn(0, 0, width, height);
        var hole = Native.CreateRectRgn(rail, title, rail + wr.Width, title + wr.Height);
        Native.CombineRgn(full, full, hole, 4);
        Native.SetWindowRgn(_ourWindow, full, true);
        Native.DeleteObject(hole);
    }

    public static void StopDock()
    {
        if (_hook != IntPtr.Zero)
        {
            Native.UnhookWinEvent(_hook);
            _hook = IntPtr.Zero;
        }
        _dockTimer?.Stop();
        _dockTimer = null;
        _proc = null;
        if (_ourWindow != IntPtr.Zero)
            Native.SetWindowRgn(_ourWindow, IntPtr.Zero, true);
    }

    public static CapturedFrame? CaptureChat()
    {
        var hwnd = _snap?.Hwnd ?? IntPtr.Zero;
        if (hwnd == IntPtr.Zero || !Native.IsWindow(hwnd)) return null;
        if (!Native.GetClientRect(hwnd, out var rc) || rc.Width < 50 || rc.Height < 50) return null;
        var cropLeft = rc.Width > 700 ? Math.Min(320, rc.Width / 3) : 0;
        return WindowPixels.Capture(hwnd, cropLeft, 0, rc.Width - cropLeft, rc.Height);
    }
}

public sealed class CapturedFrame
{
    public required byte[] Bgra { get; init; }
    public int Width { get; init; }
    public int Height { get; init; }
    public int Stride { get; init; }
}

public static class WindowPixels
{
    public static CapturedFrame? Capture(IntPtr hwnd, int x, int y, int width, int height)
    {
        if (width <= 0 || height <= 0) return null;
        var hdc = Native.GetDC(hwnd);
        if (hdc == IntPtr.Zero) return null;
        var mem = Native.CreateCompatibleDC(hdc);
        var bmp = Native.CreateCompatibleBitmap(hdc, width, height);
        var old = Native.SelectObject(mem, bmp);
        Native.BitBlt(mem, 0, 0, width, height, hdc, x, y, Native.SrcCopy);
        var info = new Native.BITMAPINFO
        {
            bmiHeader = new Native.BITMAPINFOHEADER
            {
                biSize = (uint)Marshal.SizeOf<Native.BITMAPINFOHEADER>(),
                biWidth = width,
                biHeight = -height,
                biPlanes = 1,
                biBitCount = 32,
                biCompression = 0
            }
        };
        var bytes = new byte[width * height * 4];
        Native.GetDIBits(mem, bmp, 0, (uint)height, bytes, ref info, 0);
        Native.SelectObject(mem, old);
        Native.DeleteObject(bmp);
        Native.DeleteDC(mem);
        Native.ReleaseDC(hwnd, hdc);
        return new CapturedFrame { Bgra = bytes, Width = width, Height = height, Stride = width * 4 };
    }
}

public static class PasteService
{
    public static void SetClipboard(string text)
    {
        System.Windows.Clipboard.SetText(text ?? "");
    }

    public static void CtrlV()
    {
        var inputs = new Native.INPUT[4];
        inputs[0] = Key(0x11, false);
        inputs[1] = Key(0x56, false);
        inputs[2] = Key(0x56, true);
        inputs[3] = Key(0x11, true);
        Native.SendInput(4, inputs, Marshal.SizeOf<Native.INPUT>());
    }

    public static bool FocusComposer(IntPtr hwnd)
    {
        if (hwnd == IntPtr.Zero || !Native.GetClientRect(hwnd, out var rc)) return false;
        var (x, y) = ComposerFocus.SuggestPoint(rc.Width, rc.Height);
        if (!ComposerFocus.IsSafeComposerPoint(x, y, rc.Width, rc.Height)) return false;
        var pt = new Native.Point { X = x, Y = y };
        if (!Native.ClientToScreen(hwnd, ref pt)) return false;
        Native.SetForegroundWindow(hwnd);
        Native.SetCursorPos(pt.X, pt.Y);
        var down = new Native.INPUT { type = Native.InputMouse, U = new Native.InputUnion { mi = new Native.MOUSEINPUT { dwFlags = Native.MouseeventfLeftDown } } };
        var up = new Native.INPUT { type = Native.InputMouse, U = new Native.InputUnion { mi = new Native.MOUSEINPUT { dwFlags = Native.MouseeventfLeftUp } } };
        Native.SendInput(2, [down, up], Marshal.SizeOf<Native.INPUT>());
        return true;
    }

    private static Native.INPUT Key(ushort vk, bool up) => new()
    {
        type = Native.InputKeyboard,
        U = new Native.InputUnion
        {
            ki = new Native.KEYBDINPUT
            {
                wVk = vk,
                dwFlags = up ? Native.KeyeventfKeyup : 0
            }
        }
    };
}
