using System.Windows;
using System.Windows.Interop;
using System.Windows.Media;

namespace DahuoChat;

public partial class App : Application
{
    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        DispatcherUnhandledException += (_, ev) =>
        {
            WeChatEmbedder.RestoreAll();
            ev.Handled = false;
        };
        AppDomain.CurrentDomain.UnhandledException += (_, _) => WeChatEmbedder.RestoreAll();
        TaskScheduler.UnobservedTaskException += (_, _) => WeChatEmbedder.RestoreAll();
        SessionEnding += (_, _) => WeChatEmbedder.RestoreAll();

        var shots = ArgValue(e.Args, "--shots");
        var demo = shots is not null || e.Args.Any(a => a == "--demo");
        if (demo)
            RenderOptions.ProcessRenderMode = RenderMode.SoftwareOnly;

        var window = new MainWindow(new Session(demo), shots);
        MainWindow = window;
        window.Show();
    }

    protected override void OnExit(ExitEventArgs e)
    {
        WeChatEmbedder.RestoreAll();
        base.OnExit(e);
    }

    private static string? ArgValue(string[] args, string name)
    {
        for (var i = 0; i < args.Length; i++)
        {
            if (args[i] == name && i + 1 < args.Length) return args[i + 1];
            if (args[i].StartsWith(name + "=", StringComparison.Ordinal))
                return args[i][(name.Length + 1)..];
        }
        return null;
    }
}
