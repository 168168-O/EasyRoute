/** In-browser stand-in for the Wails backend, used only when the desktop shell is absent. */
export const installPreviewBridge = () => {
  if (import.meta.env.PROD) return
  const runtime = (window as Window & { runtime?: { EventsOn?: unknown } }).runtime
  if (runtime?.EventsOn) return

  const files: Record<string, string> = {}
  const ok = (data?: unknown) => Promise.resolve({ flag: true, data: data == null ? '' : String(data) })
  const processes = [
    { name: 'Google Chrome', exe: 'chrome.exe' },
    { name: 'Telegram', exe: 'Telegram.exe' },
    { name: 'Discord', exe: 'Discord.exe' },
    { name: 'Microsoft Edge', exe: 'msedge.exe' },
    { name: 'notepad', exe: 'notepad.exe' },
  ]
  const app: Record<string, (...args: unknown[]) => Promise<unknown>> = {
    GetEnv: () =>
      Promise.resolve({
        appName: '达货爱vpn姑娘.exe',
        appVersion: 'v1.0.0',
        basePath: 'C:/达货爱vpn姑娘',
        os: 'windows',
        arch: 'amd64',
        isPrivileged: true,
      }),
    IsStartup: () => Promise.resolve(false),
    ReadFile: (path) =>
      Object.prototype.hasOwnProperty.call(files, String(path))
        ? ok(files[String(path)])
        : Promise.resolve({ flag: false, data: 'not found' }),
    WriteFile: (path, content) => {
      files[String(path)] = String(content)
      return ok('ok')
    },
    FileExists: () => ok('false'),
    RemoveFile: () => ok('ok'),
    CopyFile: () => ok('ok'),
    MakeDir: () => ok('ok'),
    ListProcesses: () => ok(JSON.stringify(processes)),
    ListDouyinExes: () => ok('[]'),
    PickFile: () => Promise.resolve({ flag: false, data: 'cancelled' }),
    BackgroundVideoURL: () => Promise.resolve({ flag: false, data: 'none' }),
    Translate: (text, target) => {
      const raw = String(text || '').trim()
      if (!raw) return Promise.resolve({ flag: false, data: '请先输入要翻译的内容' })
      const chinese = /[\u4e00-\u9fff]/.test(raw)
      const chosen = String(target || 'auto')
      const to = chosen !== 'auto' ? chosen : chinese ? 'en' : 'zh-Hans'
      const translated = chinese ? `[en] ${raw}` : `[中文] ${raw}`
      return ok(JSON.stringify({ text: translated, sourceLang: chinese ? 'zh-Hans' : 'en', targetLang: to, provider: 'bing' }))
    },
    GetInterfaces: () => ok(''),
    GetSystemProxy: () => Promise.resolve({ flag: false, data: '' }),
    GetSystemProxyBypass: () => ok(''),
  }

  ;(window as unknown as { __DHAGN_PREVIEW__?: boolean }).__DHAGN_PREVIEW__ = true
  ;(window as unknown as { go: unknown }).go = {
    bridge: {
      App: new Proxy(app, {
        get(target, prop: string) {
          if (prop in target) return target[prop]
          return () => ok('')
        },
      }),
    },
  }
  ;(window as unknown as { runtime: unknown }).runtime = new Proxy(
    {},
    {
      get(_target, prop) {
        if (prop === 'EventsOn' || prop === 'EventsOnMultiple' || prop === 'EventsOnce') {
          return () => () => undefined
        }
        if (prop === 'WindowIsMaximised' || prop === 'WindowIsMinimised' || prop === 'WindowIsFullscreen') {
          return () => Promise.resolve(false)
        }
        if (prop === 'WindowGetSize') return () => Promise.resolve({ w: 1280, h: 800 })
        if (prop === 'ClipboardGetText') return () => Promise.resolve('Where is the station?')
        if (prop === 'ClipboardSetText') return () => Promise.resolve()
        if (prop === 'WindowGetPosition') return () => Promise.resolve({ x: 0, y: 0 })
        return () => Promise.resolve()
      },
    },
  )
}

installPreviewBridge()
