namespace DahuoChat;

public partial class ReplyPanel
{
    public ReplyPanel()
    {
        InitializeComponent();
    }

    private Session? S => DataContext as Session;

    private void Fill_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null || sender is not System.Windows.FrameworkElement fe || fe.Tag is not int index) return;
        S.DeliverIndex(index, true);
    }

    private void Copy_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null || sender is not System.Windows.FrameworkElement fe || fe.Tag is not int index) return;
        S.DeliverIndex(index, false);
    }

    private void Tone_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null || sender is not System.Windows.FrameworkElement fe) return;
        S.SetTone(fe.Tag as string ?? Tones.Default);
    }

    private void Expand_Click(object sender, System.Windows.RoutedEventArgs e) => S?.ToggleExpand();
    private void Batch_Click(object sender, System.Windows.RoutedEventArgs e) => _ = S?.SuggestAsync(SuggestAdjust.AnotherBatch);
    private void Warmer_Click(object sender, System.Windows.RoutedEventArgs e) => _ = S?.SuggestAsync(SuggestAdjust.Warmer);
    private void Shorter_Click(object sender, System.Windows.RoutedEventArgs e) => _ = S?.SuggestAsync(SuggestAdjust.Shorter);
    private void Polish_Click(object sender, System.Windows.RoutedEventArgs e) => _ = S?.SuggestAsync(SuggestAdjust.Polish);
    private void Drawer_Click(object sender, System.Windows.RoutedEventArgs e) => S?.ToggleDrawer();
    private void Collapse_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        S.PanelOpen = !S.PanelOpen;
    }

    private void ModeSuggest_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        S.PolishMode = false;
    }

    private void ModePolish_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        S.PolishMode = true;
    }

    private void OpenSettings_Click(object sender, System.Windows.RoutedEventArgs e) => S?.OpenSettings();

    private void UseScript_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null || sender is not System.Windows.FrameworkElement fe) return;
        S.UseScript(fe.Tag as string ?? "");
    }
}
