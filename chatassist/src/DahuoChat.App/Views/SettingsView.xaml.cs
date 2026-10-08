using Microsoft.Win32;

namespace DahuoChat;

public partial class SettingsView
{
    private bool _ready;

    public SettingsView()
    {
        InitializeComponent();
        DataContextChanged += (_, _) => Wire();
        Loaded += (_, _) => Wire();
    }

    private void Wire()
    {
        if (_ready || DataContext is not Session s) return;
        KeyBox.Password = s.Demo ? "" : s.ApiKey;
        CountBox.SelectedIndex = Math.Clamp(s.Settings.SuggestionCount, 3, 5) - 3;
        EmbedBox.SelectedIndex = s.Settings.EmbedMode == "dock" ? 1 : 0;
        FillBox.SelectedIndex = s.Settings.FillMode == "copy" ? 1 : 0;
        _ready = true;
    }

    private Session? S => DataContext as Session;

    private void Save_Changed(object sender, System.Windows.RoutedEventArgs e)
    {
        if (!_ready || S is null) return;
        S.ApiKey = KeyBox.Password;
        S.SaveSettingsFromUi();
        Saved?.Invoke();
    }

    private void Tone_Changed(object sender, System.Windows.Controls.SelectionChangedEventArgs e)
    {
        if (!_ready || S is null || e.AddedItems.Count == 0) return;
        if (e.AddedItems[0] is System.Windows.Controls.ComboBoxItem item)
            S.Settings.DefaultTone = item.Content?.ToString() ?? Tones.Default;
        S.SaveSettingsFromUi();
    }

    private void Count_Changed(object sender, System.Windows.Controls.SelectionChangedEventArgs e)
    {
        if (!_ready || S is null) return;
        S.Settings.SuggestionCount = CountBox.SelectedIndex + 3;
        S.SaveSettingsFromUi();
    }

    private void Embed_Changed(object sender, System.Windows.Controls.SelectionChangedEventArgs e)
    {
        if (!_ready || S is null || e.AddedItems.Count == 0) return;
        var text = (e.AddedItems[0] as System.Windows.Controls.ComboBoxItem)?.Content?.ToString();
        S.Settings.EmbedMode = text == "贴在旁边" ? "dock" : "embed";
        S.SaveSettingsFromUi();
        EmbedChanged?.Invoke(S.Settings.EmbedMode);
    }

    private void Fill_Changed(object sender, System.Windows.Controls.SelectionChangedEventArgs e)
    {
        if (!_ready || S is null || e.AddedItems.Count == 0) return;
        var text = (e.AddedItems[0] as System.Windows.Controls.ComboBoxItem)?.Content?.ToString();
        S.Settings.FillMode = text == "只复制" ? "copy" : "fill";
        S.SaveSettingsFromUi();
    }

    private void Preset_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null || sender is not System.Windows.FrameworkElement fe) return;
        S.ApplyPreset(fe.Tag as string ?? "DeepSeek");
        BaseUrlBox.Text = S.Settings.BaseUrl;
        ModelBox.Text = S.Settings.Model;
    }

    private void Test_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        S.ApiKey = KeyBox.Password;
        S.SaveSettingsFromUi();
        _ = S.TestAiAsync();
    }

    private void Cat_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null || sender is not System.Windows.FrameworkElement fe) return;
        S.ScriptCategory = fe.Tag as string ?? "问候";
    }

    private void Add_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        S.AddScript(S.ScriptCategory, NewTitle.Text, "", NewPoints.Text, "");
        NewTitle.Text = "";
        NewPoints.Text = "";
    }

    private void Import_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        var dlg = new OpenFileDialog { Filter = "表格|*.csv;*.txt" };
        if (dlg.ShowDialog() != true) return;
        S.ImportCsv(System.IO.File.ReadAllText(dlg.FileName));
    }

    private void Export_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        var dlg = new SaveFileDialog { Filter = "CSV|*.csv", FileName = "话术.csv" };
        if (dlg.ShowDialog() != true) return;
        System.IO.File.WriteAllText(dlg.FileName, S.ExportCsv());
    }

    private void Back_Click(object sender, System.Windows.RoutedEventArgs e) => S?.CloseOverlay();

    public event System.Action<string>? EmbedChanged;
    public event System.Action? Saved;
}
