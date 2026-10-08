import dns from 'node:dns/promises'
import type { IncomingMessage, ServerResponse } from 'node:http'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig, type Plugin } from 'vite'

const previewHttpAllowed = (target: string) => {
  if (PREVIEW_HTTP.has(target)) return true
  try {
    const parsed = new URL(target)
    return parsed.protocol === 'https:' && parsed.host === 'api.mymemory.translated.net' && parsed.pathname === '/get'
  } catch {
    return false
  }
}

const PREVIEW_HTTP = new Set([
  'https://myip.ipip.net',
  'http://myip.ipip.net',
  'https://ipinfo.io/json',
  'http://ip-api.com/json?lang=zh-CN',
  'https://www.baidu.com/robots.txt',
  'https://weixin.qq.com/',
  'https://www.douyin.com/robots.txt',
  'https://www.google.com/generate_204',
  'https://www.youtube.com/generate_204',
  'https://github.com/robots.txt',
  'https://web.telegram.org/',
])
const PREVIEW_DNS = new Set(['www.baidu.com', 'www.qq.com'])

const sendJson = (res: ServerResponse, status: number, body: unknown) => {
  res.statusCode = status
  res.setHeader('content-type', 'application/json')
  res.end(JSON.stringify(body))
}

const previewUA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0'

/** Bing's translator page issues a short-lived token. Cookies from that page must ride along on the POST. */
const previewBing = async (text: string, to: string) => {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 8000)
  const cookies = new Map<string, string>()
  const takeCookies = (response: Response) => {
    const lines = typeof response.headers.getSetCookie === 'function' ? response.headers.getSetCookie() : []
    for (const line of lines) {
      const pair = line.split(';')[0] || ''
      const eq = pair.indexOf('=')
      if (eq > 0) cookies.set(pair.slice(0, eq).trim(), pair.slice(eq + 1).trim())
    }
  }
  try {
    let current = 'https://www.bing.com/translator?mkt=zh-CN'
    let html = ''
    let status = 0
    for (let hop = 0; hop < 4; hop += 1) {
      const response = await fetch(current, {
        redirect: 'manual',
        signal: controller.signal,
        headers: { 'User-Agent': previewUA, Accept: 'text/html', 'Accept-Language': 'zh-CN,zh;q=0.9' },
      })
      takeCookies(response)
      if (response.status >= 300 && response.status < 400) {
        const location = response.headers.get('location')
        if (!location) break
        current = new URL(location, current).toString()
        continue
      }
      status = response.status
      html = await response.text()
      break
    }
    const key = html.match(/params_AbusePreventionHelper\s*=\s*\[(\d+),"([^"]+)"(?:,(\d+))?/)
    const ig = html.match(/IG:"([A-Fa-f0-9]+)"/)
    const iid = html.match(/data-iid="(translator\.[^"]+)"/)
    if (status !== 200 || !key || !ig || !iid) {
      return { flag: false, text: '' }
    }
    const post = new URL('https://www.bing.com/ttranslatev3')
    post.searchParams.set('isVertical', '1')
    post.searchParams.set('IG', ig[1])
    post.searchParams.set('IID', iid[1])
    const body = new URLSearchParams({
      fromLang: 'auto-detect',
      to,
      text,
      token: key[2],
      key: key[1],
    })
    const translated = await fetch(post, {
      method: 'POST',
      signal: controller.signal,
      headers: {
        'User-Agent': previewUA,
        'Content-Type': 'application/x-www-form-urlencoded',
        Referer: 'https://www.bing.com/translator',
        Cookie: [...cookies].map(([name, value]) => `${name}=${value}`).join('; '),
      },
      body,
    })
    const payload = (await translated.json()) as Array<{
      translations?: Array<{ text?: string; to?: string }>
      detectedLanguage?: { language?: string }
    }>
    const line = String(payload?.[0]?.translations?.[0]?.text || '').trim()
    if (!translated.ok || !line) return { flag: false, text: '' }
    return {
      flag: true,
      text: line,
      sourceLang: payload[0]?.detectedLanguage?.language || '',
      targetLang: payload[0]?.translations?.[0]?.to || to,
    }
  } catch {
    return { flag: false, text: '' }
  } finally {
    clearTimeout(timer)
  }
}

/** Dev-only stand-in for the Go HTTP client. The browser cannot fetch these hosts itself. */
const previewNet = (): Plugin => ({
  name: 'preview-net',
  configureServer(server) {
    server.middlewares.use(async (req: IncomingMessage, res: ServerResponse, next: () => void) => {
      const url = new URL(req.url || '/', 'http://127.0.0.1')
      if (url.pathname === '/__preview/lookup') {
        const host = url.searchParams.get('host') || ''
        if (!PREVIEW_DNS.has(host)) {
          sendJson(res, 400, { flag: false, error: 'host not allowed' })
          return
        }
        try {
          const addresses = await dns.lookup(host, { all: true })
          const address = addresses.find((item) => item.family === 4)?.address || addresses[0]?.address
          sendJson(res, 200, address ? { flag: true, address } : { flag: false, error: 'no address' })
        } catch (error) {
          sendJson(res, 200, { flag: false, error: error instanceof Error ? error.message : String(error) })
        }
        return
      }
      if (url.pathname === '/__preview/bing') {
        const text = (url.searchParams.get('text') || '').trim()
        const to = url.searchParams.get('to') || 'zh-Hans'
        if (!text || text.length > 5000) {
          sendJson(res, 400, { flag: false, text: '' })
          return
        }
        sendJson(res, 200, await previewBing(text, to))
        return
      }
      if (url.pathname === '/__preview/http') {
        const target = url.searchParams.get('url') || ''
        if (!previewHttpAllowed(target)) {
          sendJson(res, 400, { flag: false, status: 0, body: 'url not allowed' })
          return
        }
        const controller = new AbortController()
        const timeoutMs = target.includes('api.mymemory.translated.net') ? 5000 : 4000
        const timer = setTimeout(() => controller.abort(), timeoutMs)
        try {
          const response = await fetch(target, { signal: controller.signal, redirect: 'follow' })
          const landed = new URL(response.url)
          const started = new URL(target)
          if (landed.host !== started.host) {
            sendJson(res, 200, { flag: false, status: 0, body: 'redirect left the check host' })
            return
          }
          const text = await response.text()
          sendJson(res, 200, {
            flag: true,
            status: response.status,
            contentType: response.headers.get('content-type') || 'text/plain',
            body: text.slice(0, 8000),
          })
        } catch (error) {
          sendJson(res, 200, {
            flag: false,
            status: 0,
            body: error instanceof Error ? error.message : String(error),
          })
        } finally {
          clearTimeout(timer)
        }
        return
      }
      next()
    })
  },
})

// https://vitejs.dev/config/
export default defineConfig({
  base: './',
  plugins: [vue(), previewNet()],
  server: {
    watch: {
      ignored: ['**/node_modules/**', '**/.git/**', '**/dist/**', '**/src/bridge/wailsjs/**'],
    },
  },
  resolve: {
    extensions: ['.ts', '.js'],
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@wails': fileURLToPath(new URL('./src/bridge/wailsjs', import.meta.url)),
      vue: 'vue/dist/vue.esm-bundler.js',
    },
  },
  build: {
    cssCodeSplit: false,
    chunkSizeWarningLimit: 4096, // 4MB
    rolldownOptions: {
      output: {
        strictExecutionOrder: true,
        codeSplitting: {
          groups: [
            { name: 'vue', test: /node_modules\/vue/ },
            { name: 'codemirror', test: /node_modules\/@codemirror/ },
            { name: 'prettier', test: /node_modules\/prettier/ },
            { name: 'vendor', test: /node_modules/ },
            { name: 'index' },
          ],
        },
      },
    },
  },
})
