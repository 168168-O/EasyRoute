using System.ComponentModel;
using System.Globalization;
using System.Runtime.CompilerServices;
using System.Windows;
using System.Windows.Data;
using System.Windows.Media;

namespace DahuoChat;

public abstract class NotifyBase : INotifyPropertyChanged
{
    public event PropertyChangedEventHandler? PropertyChanged;

    protected bool Set<T>(ref T field, T value, [CallerMemberName] string? name = null)
    {
        if (EqualityComparer<T>.Default.Equals(field, value)) return false;
        field = value;
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(name));
        return true;
    }

    public void Raise(string name) => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(name));
}

public enum ScreenKind
{
    Wechat,
    Douyin,
    Wizard,
    Settings
}

public sealed class AccountVm : NotifyBase
{
    private bool _selected;
    private bool _sleeping;
    private string _badge = "";
    private string _sub = "";
    public string Id { get; set; } = "";
    public string Platform { get; set; } = "";
    public string Title { get; set; } = "";
    public string Sub { get => _sub; set => Set(ref _sub, value); }
    public string Badge { get => _badge; set => Set(ref _badge, value); }
    public bool IceBadge { get; set; }
    public bool Sleeping { get => _sleeping; set => Set(ref _sleeping, value); }
    public string Brand { get; set; } = "";
    public bool Selected { get => _selected; set => Set(ref _selected, value); }
    public bool IsAdd { get; set; }
    public string Mark => Platform == "wechat" ? "微" : Platform == "douyin" ? "抖" : "+";
}

public sealed class ReplyVm
{
    public int Index { get; init; }
    public string Tag { get; init; } = "";
    public string Text { get; init; } = "";
    public string Why { get; init; } = "";
    public bool Best { get; init; }
    public string Hotkey { get; init; } = "";
    public Brush TagBg { get; init; } = Brushes.Transparent;
    public Brush TagFg { get; init; } = Brushes.White;
}

public sealed class BoolVisConverter : IValueConverter
{
    public object Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        var on = value is true;
        if (parameter as string == "inv") on = !on;
        return on ? Visibility.Visible : Visibility.Collapsed;
    }

    public object ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => Binding.DoNothing;
}

public sealed class ScriptVm
{
    public string Id { get; init; } = "";
    public int Number { get; init; }
    public string Title { get; init; } = "";
    public string Points { get; init; } = "";
    public string Use { get; init; } = "";
    public string Category { get; init; } = "";
}
