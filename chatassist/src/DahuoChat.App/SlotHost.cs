using System.Runtime.InteropServices;
using System.Windows.Interop;

namespace DahuoChat;

public sealed class SlotHost : HwndHost
{
    public SlotHost()
    {
        SizeChanged += (_, _) => WeChatEmbedder.Fit(SafeHandle());
    }

    protected override HandleRef BuildWindowCore(HandleRef parent)
    {
        const int style = 0x40000000 | 0x10000000 | 0x02000000 | 0x04000000;
        var hwnd = Native.CreateWindowEx(0, "Static", "", style, 0, 0, 10, 10, parent.Handle, IntPtr.Zero, Native.GetModuleHandle(null), IntPtr.Zero);
        if (hwnd == IntPtr.Zero)
            throw new InvalidOperationException("没能做出微信的位子。");
        return new HandleRef(this, hwnd);
    }

    protected override void DestroyWindowCore(HandleRef hwnd)
    {
        WeChatEmbedder.RestoreAll();
        Native.DestroyWindow(hwnd.Handle);
    }

    public IntPtr SafeHandle()
    {
        try { return Handle; }
        catch (InvalidOperationException) { return IntPtr.Zero; }
    }
}
