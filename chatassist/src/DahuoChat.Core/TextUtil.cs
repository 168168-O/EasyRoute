using System.Globalization;

namespace DahuoChat;

public static class TextUtil
{
    public static int Runes(string? text)
    {
        if (string.IsNullOrEmpty(text)) return 0;
        return new StringInfo(text).LengthInTextElements;
    }

    public static string TruncateRunes(string? text, int max)
    {
        if (string.IsNullOrEmpty(text) || max <= 0) return "";
        var info = new StringInfo(text);
        if (info.LengthInTextElements <= max) return text;
        return info.SubstringByTextElements(0, max);
    }
}
