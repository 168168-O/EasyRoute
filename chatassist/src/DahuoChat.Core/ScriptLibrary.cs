using System.Text;
using System.Text.Json;

namespace DahuoChat;

public static class ScriptCategories
{
    public static readonly string[] All = ["问候", "安慰", "拒绝", "道歉", "感谢", "夸赞", "谈生意"];
}

public sealed class ScriptItem
{
    public string Id { get; set; } = Guid.NewGuid().ToString("N");
    public string Category { get; set; } = "问候";
    public string Title { get; set; } = "";
    public string Triggers { get; set; } = "";
    public List<string> Points { get; set; } = [];
    public string Sample { get; set; } = "";
    public string Avoid { get; set; } = "";
    public int Used { get; set; }
    public string LastUsed { get; set; } = "";
}

public sealed class ScriptLibrary
{
    public List<ScriptItem> Items { get; set; } = [];

    public IEnumerable<ScriptItem> In(string category)
        => Items.Where(i => i.Category == category);

    public IReadOnlyList<string> RelevantPoints(string message, string? category, int take = 3)
    {
        var q = message ?? "";
        var ranked = Items
            .Select(item => new { item, score = Score(item, q, category) })
            .Where(x => x.score > 0)
            .OrderByDescending(x => x.score)
            .Take(take)
            .SelectMany(x => x.item.Points)
            .Where(p => !string.IsNullOrWhiteSpace(p))
            .Distinct()
            .Take(8)
            .ToList();
        return ranked;
    }

    public static int Score(ScriptItem item, string message, string? category)
    {
        var score = 0;
        if (!string.IsNullOrWhiteSpace(category) && item.Category == category) score += 2;
        foreach (var trigger in SplitTriggers(item.Triggers))
        {
            if (trigger.Length >= 1 && message.Contains(trigger, StringComparison.OrdinalIgnoreCase))
                score += 3;
        }
        if (score == 0 && !string.IsNullOrWhiteSpace(category) && item.Category == category)
            score = 1;
        return score;
    }

    public static IEnumerable<string> SplitTriggers(string? triggers)
    {
        if (string.IsNullOrWhiteSpace(triggers)) yield break;
        foreach (var part in triggers.Split(['|', ',', '，', '、', ';', '；'], StringSplitOptions.RemoveEmptyEntries))
        {
            var t = part.Trim();
            if (t.Length > 0) yield return t;
        }
    }

    public string ToJson() => JsonSerializer.Serialize(Items, JsonOpts());

    public static ScriptLibrary FromJson(string? json)
    {
        var lib = new ScriptLibrary();
        if (string.IsNullOrWhiteSpace(json)) return lib;
        var items = JsonSerializer.Deserialize<List<ScriptItem>>(json, JsonOpts());
        if (items is not null) lib.Items = items;
        return lib;
    }

    public string ToCsv()
    {
        var sb = new StringBuilder();
        sb.Append('\uFEFF');
        sb.AppendLine("分类,标题,触发词,要点,参考说法,别这样说");
        foreach (var item in Items)
        {
            sb.Append(Cell(item.Category)).Append(',')
                .Append(Cell(item.Title)).Append(',')
                .Append(Cell(item.Triggers)).Append(',')
                .Append(Cell(string.Join("|", item.Points))).Append(',')
                .Append(Cell(item.Sample)).Append(',')
                .Append(Cell(item.Avoid)).AppendLine();
        }
        return sb.ToString();
    }

    public static ScriptLibrary FromCsv(string csv)
    {
        var lib = new ScriptLibrary();
        if (string.IsNullOrWhiteSpace(csv)) return lib;
        var text = csv.TrimStart('\uFEFF');
        var rows = ParseRows(text);
        var first = true;
        foreach (var row in rows)
        {
            if (row.Count == 0) continue;
            if (first)
            {
                first = false;
                if (row[0].Contains("分类")) continue;
            }
            string Col(int i) => i < row.Count ? row[i].Trim() : "";
            var category = Col(0);
            if (!ScriptCategories.All.Contains(category)) category = "问候";
            var points = Col(3).Split('|', StringSplitOptions.RemoveEmptyEntries).Select(p => p.Trim()).Where(p => p.Length > 0).ToList();
            lib.Items.Add(new ScriptItem
            {
                Category = category,
                Title = Col(1),
                Triggers = Col(2),
                Points = points,
                Sample = Col(4),
                Avoid = Col(5)
            });
        }
        return lib;
    }

    public static ScriptLibrary CreateSeed()
    {
        var lib = new ScriptLibrary();
        void Add(string cat, string title, string triggers, string[] points, string avoid)
        {
            lib.Items.Add(new ScriptItem
            {
                Category = cat,
                Title = title,
                Triggers = triggers,
                Points = points.ToList(),
                Avoid = avoid
            });
        }
        Add("问候", "久别重逢", "好久不见|在吗", ["先高兴地接住", "问问近况", "不客套"], "别只回在的");
        Add("安慰", "被当众批评", "骂|累|委屈", ["先站他那边", "不讲道理", "给他一个出口"], "别劝他忍着或辞职");
        Add("拒绝", "朋友邀约去不了", "来啊|暖房|有空吗|周末", ["先谢谢对方想着我", "说清真实原因", "主动约下次"], "别编借口，别扫兴");
        Add("拒绝", "不想借钱", "借钱|周转", ["先表示理解", "说自己也紧", "不给模糊希望"], "别说再看看然后消失");
        Add("拒绝", "推掉多余的活", "帮忙|加班|顺便", ["肯定这件事重要", "说手头排期", "给个替代方案"], "别硬扛然后耽误");
        Add("拒绝", "不想喝酒", "喝一个|干杯", ["开车或身体原因", "以茶代酒", "话题转开"], "别干了再后悔");
        Add("道歉", "工作出错", "怎么搞的|数据|抱歉", ["先认错不找理由", "给出具体补救和时间"], "别解释，别推给别人");
        Add("感谢", "别人帮了忙", "谢谢|谢啦", ["把谢意说具体", "提对方做的那件事"], "别只回不客气");
        Add("夸赞", "夸具体的用心", "装好了|好看|厉害", ["夸具体做了什么", "不说空话"], "别说你太棒了");
        Add("谈生意", "客户嫌贵", "贵|便宜|拼多多", ["先认同货比三家", "用赠品不乱降价", "感谢回购"], "别贬低别家，别编成本价");
        return lib;
    }

    private static string Cell(string? value)
    {
        var v = value ?? "";
        if (v.Contains('"') || v.Contains(',') || v.Contains('\n') || v.Contains('\r'))
            return "\"" + v.Replace("\"", "\"\"") + "\"";
        return v;
    }

    public static List<List<string>> ParseRows(string csv)
    {
        var rows = new List<List<string>>();
        var row = new List<string>();
        var cell = new StringBuilder();
        var quoted = false;
        for (var i = 0; i < csv.Length; i++)
        {
            var c = csv[i];
            if (quoted)
            {
                if (c == '"')
                {
                    if (i + 1 < csv.Length && csv[i + 1] == '"') { cell.Append('"'); i++; }
                    else quoted = false;
                }
                else cell.Append(c);
                continue;
            }
            if (c == '"') { quoted = true; continue; }
            if (c == ',') { row.Add(cell.ToString()); cell.Clear(); continue; }
            if (c == '\n')
            {
                row.Add(cell.ToString());
                cell.Clear();
                rows.Add(row);
                row = [];
                continue;
            }
            if (c == '\r') continue;
            cell.Append(c);
        }
        if (cell.Length > 0 || row.Count > 0)
        {
            row.Add(cell.ToString());
            rows.Add(row);
        }
        return rows;
    }

    private static JsonSerializerOptions JsonOpts() => new() { WriteIndented = true };
}
