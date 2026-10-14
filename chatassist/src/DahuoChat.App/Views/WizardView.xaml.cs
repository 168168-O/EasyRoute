namespace DahuoChat;

public partial class WizardView
{
    public WizardView() => InitializeComponent();
    private Session? S => DataContext as Session;

    private void PickWechat_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        S.WizardPlatform = "wechat";
        S.WizardName = "海外号";
    }

    private void PickDouyin_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        S.WizardPlatform = "douyin";
        if (S.WizardName == "海外号") S.WizardName = "店铺号";
    }

    private void Brand_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null || sender is not System.Windows.FrameworkElement fe) return;
        S.WizardBrand = fe.Tag as string ?? "Auto";
    }

    private void Name_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null || sender is not System.Windows.FrameworkElement fe) return;
        S.WizardName = fe.Tag as string ?? "";
    }

    private void Back_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        if (S is null) return;
        if (!S.CanGoBack)
        {
            S.Tell("还没有账号，先点完成。");
            return;
        }
        S.CloseOverlay();
    }

    private void Finish_Click(object sender, System.Windows.RoutedEventArgs e)
    {
        var created = S?.FinishWizard();
        if (created is not null)
            Finished?.Invoke(created);
    }

    public event System.Action<AccountRecord>? Finished;
}
