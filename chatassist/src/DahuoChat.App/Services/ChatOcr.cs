using System.Runtime.InteropServices.WindowsRuntime;
using Windows.Graphics.Imaging;
using Windows.Media.Ocr;
using Windows.Security.Cryptography;

namespace DahuoChat;

public static class ChatOcr
{
    public static async Task<IReadOnlyList<ChatLine>> ReadAsync(CapturedFrame frame)
    {
        var engine = OcrEngine.TryCreateFromLanguage(new Windows.Globalization.Language("zh-Hans"));
        if (engine is null) return [];
        var buffer = CryptographicBuffer.CreateFromByteArray(frame.Bgra);
        var bitmap = SoftwareBitmap.CreateCopyFromBuffer(buffer, BitmapPixelFormat.Bgra8, frame.Width, frame.Height);
        var result = await engine.RecognizeAsync(bitmap);
        var blocks = new List<OcrBlock>();
        foreach (var line in result.Lines)
        {
            if (string.IsNullOrWhiteSpace(line.Text) || line.Words.Count == 0) continue;
            double left = double.MaxValue, top = double.MaxValue, right = 0, bottom = 0;
            foreach (var word in line.Words)
            {
                var box = word.BoundingRect;
                left = Math.Min(left, box.X);
                top = Math.Min(top, box.Y);
                right = Math.Max(right, box.X + box.Width);
                bottom = Math.Max(bottom, box.Y + box.Height);
            }
            var cx = (int)Math.Clamp((left + right) / 2, 0, frame.Width - 1);
            var cy = (int)Math.Clamp((top + bottom) / 2, 0, frame.Height - 1);
            var i = cy * frame.Stride + cx * 4;
            byte b = 255, g = 255, r = 255;
            if (i + 2 < frame.Bgra.Length)
            {
                b = frame.Bgra[i];
                g = frame.Bgra[i + 1];
                r = frame.Bgra[i + 2];
            }
            blocks.Add(new OcrBlock(line.Text.Trim(), left, top, Math.Max(1, right - left), Math.Max(1, bottom - top), r, g, b));
        }
        return ChatParser.Parse(blocks, frame.Width);
    }
}
