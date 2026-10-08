# 达货聊天

给电脑微信和国内抖音用的回复参谋。窗口左边是账号，中间是聊天，右边只出建议。

它只帮你想怎么回、把句子放进输入框。发送永远由你自己按。没有群发，没有自动回复，没有定时发送。

## 怎么用

1. 双击 `dahuo-chat.exe`。不用另外安装运行库，电脑上要有微软网页运行环境（一般随浏览器已经在）。
2. 微信：先自己打开并登录电脑版微信，再在左边点「微信」。程序会把已经打开的微信窗口放进来。觉得不对，点标题栏「显示不对」，或到设置里改成「贴在旁边」。
3. 抖音：点「添加」，用手机抖音扫页面自己的二维码。每个号单独一个文件夹，登录不互通。只打开 douyin.com。
4. 右边出三条建议。点「填入」会粘贴到输入框，不会替你发出去。点「复制」只进剪贴板。`Alt+1` 到 `Alt+5` 是填入，`Ctrl+Alt+R` 是再读一遍对方的话。
5. 设置里填写 AI 钥匙。可选 DeepSeek、智谱、通义、Kimi，也可以填兼容接口。钥匙用系统加密存在本机。

答应你：只给建议，永远由你自己按发送；没有群发，没有自动回复。

## 数据放哪

`%LOCALAPPDATA%\DahuoChat\`

里面有设置、话术、语气记忆、抖音各号的独立文件夹。不保存聊天截图。

## 开发

需要 .NET 8 SDK。

```bash
dotnet test chatassist/DahuoChat.sln -c Release
dotnet publish chatassist/src/DahuoChat.App/DahuoChat.App.csproj -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true -p:EnableCompressionInSingleFile=true -o chatassist/publish
```

演示画面（没有微信时用来看界面）：

```bash
dahuo-chat.exe --demo
dahuo-chat.exe --shots 输出目录
```

## 真机上还要看的

微信放进来和退出后还原、贴在旁边、中文识别、抖音扫码登录、休眠后的内存，都要在你自己的电脑上试。自动化构建里没有微信，也没有真实的抖音登录。
