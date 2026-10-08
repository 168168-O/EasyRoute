namespace DahuoChat;

public enum Speaker
{
    Them,
    Me
}

public readonly record struct OcrBlock(string Text, double X, double Y, double Width, double Height, byte R, byte G, byte B)
{
    public double CenterX => X + Width / 2;
    public double CenterY => Y + Height / 2;
}

public sealed record ChatLine(Speaker Who, string Text, double Y);

public static class ChatParser
{
    public static IReadOnlyList<ChatLine> Parse(IReadOnlyList<OcrBlock> blocks, double frameWidth)
    {
        if (blocks.Count == 0 || frameWidth <= 0) return [];
        var ordered = blocks
            .Where(b => !string.IsNullOrWhiteSpace(b.Text))
            .OrderBy(b => b.Y)
            .ThenBy(b => b.X)
            .ToList();

        var lines = new List<ChatLine>();
        foreach (var group in Group(ordered))
        {
            var who = group.Any(b => IsOwnBubble(b, frameWidth)) ? Speaker.Me
                : group.Average(b => b.CenterX) < frameWidth * 0.55 ? Speaker.Them : Speaker.Me;
            var text = string.Concat(group.OrderBy(b => b.X).Select(b => b.Text.Trim()));
            if (text.Length == 0) continue;
            if (lines.Count > 0 && lines[^1].Who == who && Math.Abs(group[0].Y - lines[^1].Y) < 8)
            {
                var prev = lines[^1];
                lines[^1] = prev with { Text = prev.Text + text };
            }
            else
            {
                lines.Add(new ChatLine(who, text, group[0].Y));
            }
        }
        return lines;
    }

    public static bool IsOwnBubble(OcrBlock block, double frameWidth)
    {
        var green = block.G >= 150 && block.G > block.R + 25 && block.G > block.B + 25;
        if (green) return true;
        var blue = block.B >= 150 && block.B > block.R + 20 && block.B > block.G;
        return blue && block.CenterX >= frameWidth * 0.5;
    }

    public static bool ShouldSuggest(IReadOnlyList<ChatLine> lines)
        => lines.Count > 0 && lines[^1].Who == Speaker.Them;

    private static IEnumerable<List<OcrBlock>> Group(List<OcrBlock> ordered)
    {
        var current = new List<OcrBlock>();
        foreach (var block in ordered)
        {
            if (current.Count == 0)
            {
                current.Add(block);
                continue;
            }
            var prev = current[^1];
            var sameBand = Math.Abs(block.CenterY - prev.CenterY) < Math.Max(18, prev.Height * 0.8);
            var close = block.X <= prev.X + prev.Width + 24;
            if (sameBand && close) current.Add(block);
            else
            {
                yield return current;
                current = [block];
            }
        }
        if (current.Count > 0) yield return current;
    }
}

public static class ThumbnailHash
{
    public static ulong Average(byte[] bgra, int width, int height, int stride)
    {
        if (bgra.Length == 0 || width <= 0 || height <= 0) return 0;
        const int n = 8;
        ulong bits = 0;
        long sum = 0;
        Span<byte> samples = stackalloc byte[n * n];
        for (var y = 0; y < n; y++)
        {
            var sy = Math.Clamp((int)((y + 0.5) * height / n), 0, height - 1);
            for (var x = 0; x < n; x++)
            {
                var sx = Math.Clamp((int)((x + 0.5) * width / n), 0, width - 1);
                var i = sy * stride + sx * 4;
                if (i + 2 >= bgra.Length) continue;
                var gray = (byte)((bgra[i] + bgra[i + 1] + bgra[i + 2]) / 3);
                samples[y * n + x] = gray;
                sum += gray;
            }
        }
        var avg = (byte)(sum / (n * n));
        bits = (ulong)avg << 56;
        for (var i = 0; i < 56; i++)
        {
            if (samples[i] >= 128) bits |= 1UL << i;
        }
        return bits;
    }

    public static bool Changed(ulong before, ulong after) => before != after;
}

public static class ObserverMessages
{
    public static bool TryParse(string? json, out string who, out string text, out int? unread)
    {
        who = "";
        text = "";
        unread = null;
        if (string.IsNullOrWhiteSpace(json)) return false;
        try
        {
            using var doc = System.Text.Json.JsonDocument.Parse(json);
            var root = doc.RootElement;
            var type = root.TryGetProperty("type", out var t) ? t.GetString() : "";
            if (type == "unread" && root.TryGetProperty("count", out var c) && c.TryGetInt32(out var n))
            {
                unread = Math.Max(0, n);
                return true;
            }
            if (type == "message")
            {
                who = root.TryGetProperty("who", out var w) ? w.GetString() ?? "" : "";
                text = root.TryGetProperty("text", out var tx) ? tx.GetString() ?? "" : "";
                text = text.Trim();
                return who == "them" && text.Length > 0 && text.Length <= 500;
            }
        }
        catch (System.Text.Json.JsonException)
        {
            return false;
        }
        return false;
    }
}
