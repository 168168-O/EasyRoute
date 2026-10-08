using System.Windows.Media;

namespace DahuoChat;

public static class AppFont
{
    public static FontFamily Pick()
    {
        var path = Environment.GetEnvironmentVariable("DAHUO_FONT");
        if (string.IsNullOrWhiteSpace(path) || !File.Exists(path))
            return new FontFamily("Microsoft YaHei UI, Microsoft YaHei, Segoe UI");
        var dir = Path.GetDirectoryName(Path.GetFullPath(path)) ?? ".";
        if (!dir.EndsWith(Path.DirectorySeparatorChar))
            dir += Path.DirectorySeparatorChar;
        var family = Environment.GetEnvironmentVariable("DAHUO_FONT_FAMILY");
        if (string.IsNullOrWhiteSpace(family)) family = "Noto Sans SC";
        return new FontFamily(new Uri(dir), "./#" + family);
    }
}
