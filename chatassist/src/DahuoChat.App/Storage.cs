using System.Text.Json;

namespace DahuoChat;

public sealed class PersistedSettings
{
    public bool AutoSuggest { get; set; } = true;
    public string DefaultTone { get; set; } = Tones.Default;
    public bool RememberTone { get; set; } = true;
    public int SuggestionCount { get; set; } = 3;
    public int SleepMinutes { get; set; } = 10;
    public string Hotkey { get; set; } = "Ctrl+Alt+R";
    public string BaseUrl { get; set; } = AiPreset.DeepSeek.BaseUrl;
    public string Model { get; set; } = AiPreset.DeepSeek.Model;
    public string ApiKeyProtected { get; set; } = "";
    public string Persona { get; set; } = PromptBuilder.DefaultPersona;
    public string EmbedMode { get; set; } = "embed";
    public string FillMode { get; set; } = "fill";
    public string PresetName { get; set; } = "DeepSeek";
}

public sealed class AccountRecord
{
    public string Id { get; set; } = "";
    public string Platform { get; set; } = "";
    public string Name { get; set; } = "";
    public string Sub { get; set; } = "";
    public string Brand { get; set; } = "Edge";
}

public sealed class AccountFile
{
    public bool FirstRunDone { get; set; }
    public List<AccountRecord> Accounts { get; set; } = [];
}

public static class AppPaths
{
    public static string Root => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "DahuoChat");

    public static string SettingsFile => Path.Combine(Root, "settings.json");
    public static string ToneFile => Path.Combine(Root, "contact_tone.json");
    public static string ScriptFile => Path.Combine(Root, "scripts.json");
    public static string AccountFile => Path.Combine(Root, "accounts.json");
    public static string DouyinRoot(string id) => Path.Combine(Root, "douyin", id);
    public static string ProfileFile(string id) => Path.Combine(DouyinRoot(id), "profile.json");

    public static void Ensure() => Directory.CreateDirectory(Root);
}

public static class LocalStore
{
    private static readonly JsonSerializerOptions Opts = new() { WriteIndented = true };

    public static PersistedSettings LoadSettings()
    {
        try
        {
            if (!File.Exists(AppPaths.SettingsFile)) return new PersistedSettings();
            return JsonSerializer.Deserialize<PersistedSettings>(File.ReadAllText(AppPaths.SettingsFile), Opts) ?? new PersistedSettings();
        }
        catch
        {
            return new PersistedSettings();
        }
    }

    public static void SaveSettings(PersistedSettings settings)
    {
        AppPaths.Ensure();
        File.WriteAllText(AppPaths.SettingsFile, JsonSerializer.Serialize(settings, Opts));
    }

    public static AccountFile LoadAccounts()
    {
        try
        {
            if (!File.Exists(AppPaths.AccountFile)) return new AccountFile();
            return JsonSerializer.Deserialize<AccountFile>(File.ReadAllText(AppPaths.AccountFile), Opts) ?? new AccountFile();
        }
        catch
        {
            return new AccountFile();
        }
    }

    public static void SaveAccounts(AccountFile file)
    {
        AppPaths.Ensure();
        File.WriteAllText(AppPaths.AccountFile, JsonSerializer.Serialize(file, Opts));
    }

    public static ToneMemory LoadTones()
    {
        try
        {
            return File.Exists(AppPaths.ToneFile) ? ToneMemory.FromJson(File.ReadAllText(AppPaths.ToneFile)) : new ToneMemory();
        }
        catch
        {
            return new ToneMemory();
        }
    }

    public static void SaveTones(ToneMemory memory)
    {
        AppPaths.Ensure();
        File.WriteAllText(AppPaths.ToneFile, memory.ToJson());
    }

    public static ScriptLibrary LoadScripts()
    {
        try
        {
            if (!File.Exists(AppPaths.ScriptFile)) return ScriptLibrary.CreateSeed();
            return ScriptLibrary.FromJson(File.ReadAllText(AppPaths.ScriptFile));
        }
        catch
        {
            return ScriptLibrary.CreateSeed();
        }
    }

    public static void SaveScripts(ScriptLibrary library)
    {
        AppPaths.Ensure();
        File.WriteAllText(AppPaths.ScriptFile, library.ToJson());
    }

    public static void SaveIdentity(string id, BrowserBrand brand)
    {
        var dir = AppPaths.DouyinRoot(id);
        Directory.CreateDirectory(dir);
        var path = Path.Combine(dir, "profile.json");
        if (File.Exists(path)) return;
        var profile = IdentityProfile.Create(id, brand);
        File.WriteAllText(path, JsonSerializer.Serialize(profile, Opts));
    }

    public static BrowserBrand LoadBrand(string id, string fallback)
    {
        try
        {
            var path = AppPaths.ProfileFile(id);
            if (!File.Exists(path)) return BrowserBrands.Parse(fallback);
            var profile = JsonSerializer.Deserialize<IdentityProfile>(File.ReadAllText(path), Opts);
            return profile?.Brand ?? BrowserBrands.Parse(fallback);
        }
        catch
        {
            return BrowserBrands.Parse(fallback);
        }
    }
}

public static class SecretBox
{
    public static string Protect(string plain)
    {
        if (string.IsNullOrEmpty(plain)) return "";
        var bytes = System.Text.Encoding.UTF8.GetBytes(plain);
        var enc = System.Security.Cryptography.ProtectedData.Protect(bytes, null, System.Security.Cryptography.DataProtectionScope.CurrentUser);
        return Convert.ToBase64String(enc);
    }

    public static string Unprotect(string stored)
    {
        if (string.IsNullOrWhiteSpace(stored)) return "";
        try
        {
            var enc = Convert.FromBase64String(stored);
            var bytes = System.Security.Cryptography.ProtectedData.Unprotect(enc, null, System.Security.Cryptography.DataProtectionScope.CurrentUser);
            return System.Text.Encoding.UTF8.GetString(bytes);
        }
        catch
        {
            return "";
        }
    }
}

public static class EmbeddedJs
{
    public static string Observer() => Read("observer.js");
    public static string Focus() => Read("focus.js");

    private static string Read(string file)
    {
        var asm = typeof(EmbeddedJs).Assembly;
        var name = asm.GetManifestResourceNames().First(n => n.EndsWith(file, StringComparison.Ordinal));
        using var stream = asm.GetManifestResourceStream(name)!;
        using var reader = new StreamReader(stream);
        return reader.ReadToEnd();
    }
}
