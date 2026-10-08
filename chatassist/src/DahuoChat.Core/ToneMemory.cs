using System.Text.Json;

namespace DahuoChat;

public sealed class ToneMemory
{
    private readonly Dictionary<string, string> _map = new(StringComparer.Ordinal);
    public string DefaultTone { get; set; } = Tones.Default;
    public bool RememberPerContact { get; set; } = true;

    public static string Key(string platform, string contact)
        => (platform ?? "").Trim() + "|" + (contact ?? "").Trim();

    public string Get(string platform, string contact)
    {
        if (!RememberPerContact) return Normalize(DefaultTone);
        return _map.TryGetValue(Key(platform, contact), out var tone) ? Normalize(tone) : Normalize(DefaultTone);
    }

    public void Set(string platform, string contact, string tone)
    {
        if (!RememberPerContact) return;
        if (string.IsNullOrWhiteSpace(contact)) return;
        _map[Key(platform, contact)] = Normalize(tone);
    }

    public IReadOnlyDictionary<string, string> Snapshot() => new Dictionary<string, string>(_map);

    public string ToJson()
    {
        var dto = new ToneFile
        {
            DefaultTone = DefaultTone,
            RememberPerContact = RememberPerContact,
            Contacts = _map.Select(kv => new ToneEntry { Key = kv.Key, Tone = kv.Value }).ToList()
        };
        return JsonSerializer.Serialize(dto, JsonOpts());
    }

    public static ToneMemory FromJson(string? json)
    {
        var memory = new ToneMemory();
        if (string.IsNullOrWhiteSpace(json)) return memory;
        var dto = JsonSerializer.Deserialize<ToneFile>(json, JsonOpts());
        if (dto is null) return memory;
        memory.DefaultTone = Normalize(dto.DefaultTone);
        memory.RememberPerContact = dto.RememberPerContact;
        foreach (var entry in dto.Contacts ?? [])
        {
            if (string.IsNullOrWhiteSpace(entry.Key)) continue;
            memory._map[entry.Key] = Normalize(entry.Tone);
        }
        return memory;
    }

    public static string Normalize(string? tone)
    {
        if (string.IsNullOrWhiteSpace(tone)) return Tones.Default;
        var t = tone.Trim();
        return Tones.All.Contains(t) ? t : Tones.Default;
    }

    private static JsonSerializerOptions JsonOpts() => new() { WriteIndented = true };

    private sealed class ToneFile
    {
        public string DefaultTone { get; set; } = Tones.Default;
        public bool RememberPerContact { get; set; } = true;
        public List<ToneEntry>? Contacts { get; set; }
    }

    private sealed class ToneEntry
    {
        public string Key { get; set; } = "";
        public string Tone { get; set; } = Tones.Default;
    }
}
