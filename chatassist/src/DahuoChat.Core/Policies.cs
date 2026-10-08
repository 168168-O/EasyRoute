namespace DahuoChat;

public enum SleepActionKind
{
    Suspend,
    Peek,
    Wake
}

public sealed record TabSnapshot(string Id, bool Awake, DateTimeOffset LastActive, DateTimeOffset? LastPeek);

public sealed record SleepAction(string Id, SleepActionKind Kind);

public static class SleepPolicy
{
    public const int MaxAwake = 2;
    public static readonly TimeSpan IdleBeforeSleep = TimeSpan.FromMinutes(10);
    public static readonly TimeSpan PeekInterval = TimeSpan.FromMinutes(3);

    public static IReadOnlyList<SleepAction> Tick(IReadOnlyList<TabSnapshot> tabs, DateTimeOffset now)
    {
        var actions = new List<SleepAction>();
        var awake = tabs.Where(t => t.Awake).OrderBy(t => t.LastActive).ToList();
        foreach (var tab in awake)
        {
            if (now - tab.LastActive >= IdleBeforeSleep)
                actions.Add(new SleepAction(tab.Id, SleepActionKind.Suspend));
        }
        var stillAwake = awake.Count - actions.Count;
        if (stillAwake > MaxAwake)
        {
            foreach (var tab in awake)
            {
                if (stillAwake <= MaxAwake) break;
                if (actions.Any(a => a.Id == tab.Id)) continue;
                actions.Add(new SleepAction(tab.Id, SleepActionKind.Suspend));
                stillAwake--;
            }
        }

        foreach (var tab in tabs.Where(t => !t.Awake))
        {
            var due = tab.LastPeek is null || now - tab.LastPeek >= PeekInterval;
            if (due) actions.Add(new SleepAction(tab.Id, SleepActionKind.Peek));
        }
        return actions;
    }
}

public static class EmbedHealth
{
    public static bool ShouldFallback(bool windowAlive, int width, int height, int foreignFocusStreak, bool userReportedBad)
    {
        if (userReportedBad) return true;
        if (!windowAlive) return true;
        if (width < 200 || height < 200) return true;
        if (foreignFocusStreak >= 5) return true;
        return false;
    }
}

public static class ComposerFocus
{
    public static bool IsSafeComposerPoint(int x, int y, int width, int height)
    {
        if (width < 200 || height < 200) return false;
        var inInputBand = y > height * 0.72 && y < height - 8;
        var clearOfSend = x > 24 && x < width * 0.72;
        return inInputBand && clearOfSend;
    }

    public static (int X, int Y) SuggestPoint(int width, int height)
    {
        var x = Math.Max(25, width / 3);
        var y = Math.Max((int)(height * 0.73) + 1, height - 70);
        if (y >= height - 8) y = height - 12;
        if (!IsSafeComposerPoint(x, y, width, height))
            return (-1, -1);
        return (x, y);
    }
}

public static class DouyinHosts
{
    private static readonly string[] Suffixes =
    [
        "douyin.com",
        "douyinpic.com",
        "douyinvod.com",
        "douyinstatic.com",
        "iesdouyin.com",
        "bytedance.com",
        "byteimg.com",
        "ibyteimg.com",
        "snssdk.com",
        "zjcdn.com",
        "amemv.com",
        "pstatp.com",
        "bytegoofy.com",
        "bytecdn.cn",
        "bytednsdoc.com",
        "byteacctimg.com",
        "huoshan.com",
        "huoshanzhibo.com"
    ];

    public const string Home = "https://www.douyin.com/";

    public static bool IsAllowed(string? uri)
    {
        if (string.IsNullOrWhiteSpace(uri)) return false;
        if (uri.Equals("about:blank", StringComparison.OrdinalIgnoreCase)) return true;
        if (!Uri.TryCreate(uri, UriKind.Absolute, out var parsed)) return false;
        if (parsed.Scheme is not ("https" or "http")) return false;
        var host = parsed.Host.Trim().TrimEnd('.').ToLowerInvariant();
        if (host.Length == 0) return false;
        foreach (var suffix in Suffixes)
        {
            if (host == suffix || host.EndsWith("." + suffix, StringComparison.Ordinal))
                return true;
        }
        return false;
    }
}

public static class HotkeySpec
{
    public static bool TryParse(string? text, out uint modifiers, out uint key)
    {
        modifiers = 0;
        key = 0;
        if (string.IsNullOrWhiteSpace(text)) return false;
        var parts = text.Split('+', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
        if (parts.Length < 2) return false;
        uint mods = 0;
        string? main = null;
        foreach (var part in parts)
        {
            switch (part.ToLowerInvariant())
            {
                case "ctrl":
                case "control":
                    mods |= 0x0002;
                    break;
                case "alt":
                    mods |= 0x0001;
                    break;
                case "shift":
                    mods |= 0x0004;
                    break;
                case "win":
                    mods |= 0x0008;
                    break;
                default:
                    if (main is not null) return false;
                    main = part;
                    break;
            }
        }
        if (main is null || mods == 0) return false;
        if (!TryKey(main, out key)) return false;
        modifiers = mods;
        return true;
    }

    public static bool Conflicts(string? a, string? b)
        => !string.IsNullOrWhiteSpace(a) && string.Equals(a.Replace(" ", ""), b?.Replace(" ", ""), StringComparison.OrdinalIgnoreCase);

    private static bool TryKey(string token, out uint key)
    {
        key = 0;
        if (token.Length == 1)
        {
            var c = char.ToUpperInvariant(token[0]);
            if (c is >= 'A' and <= 'Z') { key = c; return true; }
            if (c is >= '0' and <= '9') { key = c; return true; }
        }
        if (token.StartsWith('F') && int.TryParse(token[1..], out var fn) && fn is >= 1 and <= 24)
        {
            key = (uint)(0x70 + fn - 1);
            return true;
        }
        return false;
    }
}

public static class WeChatPaths
{
    public static IEnumerable<string> Candidates(IEnumerable<string?> registryValues, IEnumerable<string> roots)
    {
        var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var value in registryValues)
        {
            if (string.IsNullOrWhiteSpace(value)) continue;
            foreach (var name in new[] { "Weixin.exe", "WeChat.exe" })
            {
                var path = Path.Combine(value.Trim().Trim('"'), name);
                if (seen.Add(path)) yield return path;
            }
        }
        foreach (var root in roots)
        {
            foreach (var relative in new[]
            {
                Path.Combine("Tencent", "Weixin", "Weixin.exe"),
                Path.Combine("Tencent", "WeChat", "WeChat.exe"),
                Path.Combine("Tencent", "WeChat", "Weixin.exe")
            })
            {
                var path = Path.Combine(root, relative);
                if (seen.Add(path)) yield return path;
            }
        }
    }
}
