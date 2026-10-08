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
