using System.Text;
using System.Text.Json;

namespace DahuoChat;

public enum BrowserBrand
{
    Auto = 0,
    Edge = 1,
    Chrome = 2,
    Browser360 = 3,
    Qq = 4,
    Sogou = 5
}

public static class BrowserBrands
{
    public static readonly BrowserBrand[] Preferred = [BrowserBrand.Edge, BrowserBrand.Chrome, BrowserBrand.Browser360];
    public static readonly BrowserBrand[] Advanced = [BrowserBrand.Qq, BrowserBrand.Sogou];

    public static string Label(BrowserBrand brand) => brand switch
    {
        BrowserBrand.Auto => "自动",
        BrowserBrand.Edge => "Edge",
        BrowserBrand.Chrome => "Chrome",
        BrowserBrand.Browser360 => "360",
        BrowserBrand.Qq => "QQ",
        BrowserBrand.Sogou => "搜狗",
        _ => "Edge"
    };

    public static bool IsAdvanced(BrowserBrand brand) => brand is BrowserBrand.Qq or BrowserBrand.Sogou;

    public static string Warning(BrowserBrand brand) => brand switch
    {
        BrowserBrand.Qq => "QQ 浏览器的真实版本往往比这台电脑的内核旧，装成它反而更显眼。一般不用选。",
        BrowserBrand.Sogou => "搜狗的标识和现在的内核配不到一起，一般不用选。",
        _ => ""
    };

    public static BrowserBrand Parse(string? text)
    {
        if (string.IsNullOrWhiteSpace(text)) return BrowserBrand.Edge;
        return text.Trim().ToLowerInvariant() switch
        {
            "auto" or "自动" => BrowserBrand.Auto,
            "edge" => BrowserBrand.Edge,
            "chrome" => BrowserBrand.Chrome,
            "360" or "browser360" => BrowserBrand.Browser360,
            "qq" => BrowserBrand.Qq,
            "sogou" or "搜狗" => BrowserBrand.Sogou,
            _ => BrowserBrand.Edge
        };
    }
}

public sealed record UaBrand(string Brand, string Version);

public sealed class UserAgentProfile
{
    public BrowserBrand Brand { get; init; }
    public string UserAgent { get; init; } = "";
    public string FullVersion { get; init; } = "";
    public string Major { get; init; } = "";
    public IReadOnlyList<UaBrand> Brands { get; init; } = [];
    public IReadOnlyList<UaBrand> FullVersionList { get; init; } = [];
    public bool AdvancedWarning { get; init; }
}

public sealed class IdentityProfile
{
    public string Id { get; set; } = "";
    public BrowserBrand Brand { get; set; } = BrowserBrand.Edge;
    public string ChosenAt { get; set; } = "";

    public static IdentityProfile Create(string id, BrowserBrand brand)
    {
        var resolved = brand == BrowserBrand.Auto ? BrowserBrand.Edge : brand;
        return new IdentityProfile
        {
            Id = id,
            Brand = resolved,
            ChosenAt = DateTimeOffset.Now.ToString("yyyy-MM-dd")
        };
    }
}

public static class IdentityAssigner
{
    public static BrowserBrand PickUnused(IEnumerable<BrowserBrand> used, int salt = 0)
    {
        var taken = new HashSet<BrowserBrand>(used.Where(b => b != BrowserBrand.Auto));
        foreach (var brand in BrowserBrands.Preferred)
        {
            if (!taken.Contains(brand)) return brand;
        }
        return BrowserBrands.Preferred[Math.Abs(salt) % BrowserBrands.Preferred.Length];
    }
}

public static class UserAgentFactory
{
    public static UserAgentProfile Create(BrowserBrand brand, string? browserVersion)
    {
        var resolved = brand == BrowserBrand.Auto ? BrowserBrand.Edge : brand;
        var (major, full) = ParseVersion(browserVersion);
        var seed = int.TryParse(major, out var n) ? n : 120;
        var grease = Grease(seed);
        var product = Product(resolved, major, full);
        var ordered = Order(seed, grease, new UaBrand("Chromium", major), product.Brand);
        var fullOrdered = Order(seed,
            new UaBrand(grease.Brand, grease.Version == "8" ? "8.0.0.0" : grease.Version == "99" ? "99.0.0.0" : "24.0.0.0"),
            new UaBrand("Chromium", full),
            product.Full);

        var ua = new StringBuilder();
        ua.Append("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/");
        ua.Append(major).Append(".0.0.0 Safari/537.36");
        if (resolved == BrowserBrand.Edge)
            ua.Append(" Edg/").Append(major).Append(".0.0.0");
        else if (resolved == BrowserBrand.Qq)
            ua.Append(" QQBrowser/22.1.0");
        else if (resolved == BrowserBrand.Sogou)
            ua.Append(" SE 2.X MetaSr 1.0");

        return new UserAgentProfile
        {
            Brand = resolved,
            UserAgent = ua.ToString(),
            FullVersion = full,
            Major = major,
            Brands = ordered,
            FullVersionList = fullOrdered,
            AdvancedWarning = BrowserBrands.IsAdvanced(resolved)
        };
    }

    public static string ToCdpJson(UserAgentProfile profile)
    {
        var payload = new
        {
            userAgent = profile.UserAgent,
            acceptLanguage = "zh-CN,zh;q=0.9",
            platform = "Win32",
            userAgentMetadata = new
            {
                brands = profile.Brands.Select(b => new { brand = b.Brand, version = b.Version }),
                fullVersionList = profile.FullVersionList.Select(b => new { brand = b.Brand, version = b.Version }),
                fullVersion = profile.FullVersion,
                platform = "Windows",
                platformVersion = "10.0.0",
                architecture = "x86",
                model = "",
                mobile = false,
                bitness = "64",
                wow64 = false
            }
        };
        return JsonSerializer.Serialize(payload);
    }

    public static (string Major, string Full) ParseVersion(string? browserVersion)
    {
        if (string.IsNullOrWhiteSpace(browserVersion))
            return ("131", "131.0.0.0");
        var cut = browserVersion.Trim();
        var space = cut.IndexOf(' ');
        if (space > 0) cut = cut[..space];
        var parts = cut.Split('.', StringSplitOptions.RemoveEmptyEntries);
        if (parts.Length == 0 || !int.TryParse(parts[0], out var major) || major < 80 || major > 300)
            return ("131", "131.0.0.0");
        var full = parts.Length >= 4
            ? string.Join('.', parts.Take(4))
            : major + ".0.0.0";
        return (major.ToString(), full);
    }

    private static (UaBrand Brand, UaBrand Full) Product(BrowserBrand brand, string major, string full)
    {
        var (name, token) = brand switch
        {
            BrowserBrand.Edge => ("Microsoft Edge", "Edg"),
            BrowserBrand.Qq => ("QQBrowser", "QQBrowser"),
            BrowserBrand.Sogou => ("MetaSr", "MetaSr"),
            _ => ("Google Chrome", "Chrome")
        };
        var shortVersion = brand switch
        {
            BrowserBrand.Qq => "22",
            BrowserBrand.Sogou => "2",
            _ => major
        };
        var fullVersion = brand switch
        {
            BrowserBrand.Qq => "22.1.0.0",
            BrowserBrand.Sogou => "2.0.0.0",
            _ => full
        };
        _ = token;
        return (new UaBrand(name, shortVersion), new UaBrand(name, fullVersion));
    }

    private static UaBrand Grease(int seed)
    {
        string[] chars = [" ", "(", ":", "-", ".", "/", ")", ";", "=", "?", "_"];
        string[] versions = ["8", "99", "24"];
        var brand = "Not" + chars[Mod(seed, 11)] + "A" + chars[Mod(seed + 1, 11)] + "Brand";
        return new UaBrand(brand, versions[Mod(seed, 3)]);
    }

    private static IReadOnlyList<UaBrand> Order(int seed, UaBrand grease, UaBrand chromium, UaBrand product)
    {
        UaBrand[] items = [grease, chromium, product];
        int[][] orders =
        [
            [0, 1, 2],
            [0, 2, 1],
            [1, 0, 2],
            [1, 2, 0],
            [2, 0, 1],
            [2, 1, 0]
        ];
        var order = orders[Mod(seed, 6)];
        return [items[order[0]], items[order[1]], items[order[2]]];
    }

    private static int Mod(int value, int m)
    {
        var r = value % m;
        return r < 0 ? r + m : r;
    }
}
