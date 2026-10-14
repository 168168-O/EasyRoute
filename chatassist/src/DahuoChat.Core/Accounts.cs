using System.Text.Json;

namespace DahuoChat;

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

public readonly record struct AccountRemoval(bool Ok, AccountRecord? Removed, string? NextId, string Message)
{
    public static AccountRemoval Fail(string message) => new(false, null, null, message);

    public static AccountRemoval Done(AccountRecord removed, string? nextId, string message) => new(true, removed, nextId, message);
}

public static class AccountBook
{
    private static readonly JsonSerializerOptions Opts = new() { WriteIndented = true };

    public static AccountFile Load(string path)
    {
        try
        {
            if (!File.Exists(path)) return new AccountFile();
            return JsonSerializer.Deserialize<AccountFile>(File.ReadAllText(path), Opts) ?? new AccountFile();
        }
        catch
        {
            return new AccountFile();
        }
    }

    public static void Save(string path, AccountFile file)
    {
        var dir = Path.GetDirectoryName(path);
        if (!string.IsNullOrEmpty(dir)) Directory.CreateDirectory(dir);
        File.WriteAllText(path, JsonSerializer.Serialize(file, Opts));
    }

    public static string Label(AccountRecord account)
    {
        var name = account.Name?.Trim() ?? "";
        var sub = account.Sub?.Trim() ?? "";
        if (name.Length == 0) return sub.Length == 0 ? "这个账号" : sub;
        if (sub.Length == 0) return name;
        return name + " · " + sub;
    }

    public static string ConfirmText(AccountRecord account)
    {
        var extra = account.Platform == "douyin"
            ? "这个号在本机的独立登录文件夹会一并删掉。"
            : "微信本身不会被卸载，也不会改动。";
        return "确定移除「" + Label(account) + "」？\n只从达货聊天的账号列表里去掉，不会发出任何消息。\n" + extra;
    }

    public static AccountRemoval Remove(AccountFile file, string id)
    {
        if (file.Accounts is null) file.Accounts = [];
        if (string.IsNullOrWhiteSpace(id) || id == "add")
            return AccountRemoval.Fail("先在上面点一个要移除的账号。");
        var index = file.Accounts.FindIndex(a => string.Equals(a.Id, id, StringComparison.Ordinal));
        if (index < 0) return AccountRemoval.Fail("这个账号不在列表里。");
        var removed = file.Accounts[index];
        file.Accounts.RemoveAt(index);
        string? next = null;
        if (file.Accounts.Count > 0)
            next = file.Accounts[Math.Min(index, file.Accounts.Count - 1)].Id;
        return AccountRemoval.Done(removed, next, "已移除「" + Label(removed) + "」。");
    }

    public static bool TryDeleteDirectory(string path)
    {
        try
        {
            if (string.IsNullOrWhiteSpace(path)) return false;
            if (Directory.Exists(path)) Directory.Delete(path, recursive: true);
            return true;
        }
        catch
        {
            return false;
        }
    }
}
