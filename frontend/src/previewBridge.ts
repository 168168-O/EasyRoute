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
    LookupHost: async (host) => {
      try {
        const res = await fetch(`./__preview/lookup?host=${encodeURIComponent(String(host || ''))}`)
        const data = (await res.json()) as { flag?: boolean; address?: string; error?: string }
        return data.flag && data.address ? ok(data.address) : { flag: false, data: data.error || 'no address' }
      } catch (error) {
        return { flag: false, data: String(error) }
      }
    },
    Requests: async (...args: unknown[]) => {
      const url = String(args[1] || '')
      const options = (args[4] || {}) as { Proxy?: string; Timeout?: number }
      const timeoutMs = Math.max(1, Number(options?.Timeout) || 4) * 1000
      const controller = new AbortController()
      const timer = setTimeout(() => controller.abort(), timeoutMs)
      try {
        if (options?.Proxy && options.Proxy !== 'direct') {
          return { flag: false, status: 0, headers: {}, body: 'preview has no running core' }
        }
        const response = await fetch(`./__preview/http?url=${encodeURIComponent(url)}`, { signal: controller.signal })
        const data = (await response.json()) as {
          flag?: boolean
          status?: number
          contentType?: string
          body?: string
        }
        if (!data.flag) return { flag: false, status: 0, headers: {}, body: data.body || 'request failed' }
        return {
          flag: true,
          status: data.status || 0,
          headers: { 'Content-Type': [data.contentType || 'text/plain'] },
          body: data.body || '',
        }
      } catch (error) {
        return { flag: false, status: 0, headers: {}, body: String(error) }
      } finally {
        clearTimeout(timer)
      }
    },
    PickFile: () => Promise.resolve({ flag: false, data: 'cancelled' }),
    BackgroundVideoURL: () => Promise.resolve({ flag: false, data: 'none' }),
    Translate: async (text, target) => {
      const raw = String(text || '').trim()
      if (!raw) return { flag: false, data: '请先输入要翻译的内容' }
      const chinese = /[\u4e00-\u9fff]/.test(raw)
      const chosen = String(target || 'auto')
      const to = chosen !== 'auto' ? chosen : chinese ? 'en' : 'zh-Hans'
      const source = chinese ? 'zh-CN' : 'en'
      const targetCode = to === 'zh-Hans' ? 'zh-CN' : to === 'zh-Hant' ? 'zh-TW' : to
      const endpoint =
        'https://api.mymemory.translated.net/get?q=' +
        encodeURIComponent(raw) +
        '&langpair=' +
        encodeURIComponent(`${source}|${targetCode}`)
      try {
        const response = await fetch(`./__preview/http?url=${encodeURIComponent(endpoint)}`)
        const data = (await response.json()) as { flag?: boolean; body?: string }
        const payload = JSON.parse(data.body || '{}') as { responseData?: { translatedText?: string } }
        const translated = String(payload.responseData?.translatedText || '').trim()
        if (!data.flag || !translated || translated.includes('MYMEMORY WARNING')) {
          return { flag: false, data: '翻译失败：翻译服务没有返回可用的结果。请稍后再试。' }
        }
        return ok(
          JSON.stringify({
            text: translated,
            sourceLang: chinese ? 'zh-Hans' : source,
            targetLang: to,
            provider: 'mymemory',
          }),
        )
      } catch {
        return { flag: false, data: '翻译失败：连不上翻译服务。请检查网络后再试。' }
      }
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
