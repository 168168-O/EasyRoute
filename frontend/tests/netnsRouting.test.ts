/**
 * Live routing check inside a fresh network namespace.
 *
 * sing-box runs the config produced by applyAppRouting. Curl binaries renamed
 * to Weixin.exe / douyin.exe / notepad.exe enter through the mixed inbound.
 * A local SOCKS server counts proxy hits. Direct traffic never reaches it.
 * The namespace has no upstream network, so this only passes when the rule
 * really selects the expected outbound.
 */
import { spawn, spawnSync, type ChildProcess } from 'node:child_process'
import fs from 'node:fs'
import http from 'node:http'
import net from 'node:net'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

import { applyAppRouting, sampleBaseConfig, type PinnedRoutes } from '../src/utils/appRouting.ts'

const DEST = '1.2.3.4'
const HTTP_PORT = 18080
const SOCKS_PORT = 11080
const MIXED_PORT = 20122

const prepareNet = () => {
  const script = `
import os, socket, fcntl, struct
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
SIOCGIFFLAGS, SIOCSIFFLAGS, IFF_UP = 0x8913, 0x8914, 0x1
ifr = struct.pack('16sH14s', b'lo', 0, b'\\0' * 14)
res = fcntl.ioctl(sock.fileno(), SIOCGIFFLAGS, ifr)
flags = struct.unpack('16sH', res[:18])[1]
fcntl.ioctl(sock.fileno(), SIOCSIFFLAGS, struct.pack('16sH14s', b'lo', flags | IFF_UP, b'\\0' * 14))
idx = socket.if_nametoindex('lo')
ipb = socket.inet_aton('${DEST}')
ifa = struct.pack('BBBBI', socket.AF_INET, 32, 0, 0, idx)
def rta(typ, data):
    ln = 4 + len(data)
    pad = (4 - (ln % 4)) % 4
    return struct.pack('HH', ln, typ) + data + b'\\0' * pad
payload = ifa + rta(1, ipb) + rta(2, ipb)
hdr = struct.pack('IHHII', 16 + len(payload), 20, 1 | 0x400 | 0x200 | 4, 1, os.getpid())
nl = socket.socket(socket.AF_NETLINK, socket.SOCK_RAW, socket.NETLINK_ROUTE)
nl.bind((0, 0))
nl.send(hdr + payload)
data = nl.recv(4096)
if struct.unpack_from('IHHII', data)[1] == 2:
    err = struct.unpack_from('i', data, 16)[0]
    if err != 0:
        raise OSError(-err, os.strerror(-err))
`
  const result = spawnSync('python3', ['-c', script], { encoding: 'utf8' })
  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || 'failed to bring loopback up')
  }
}

const startSocks = (hit: { n: number }) => {
  const server = net.createServer((socket) => {
    let stage = 0
    let buf = Buffer.alloc(0)
    const onData = (chunk: Buffer) => {
      buf = Buffer.concat([buf, chunk])
      if (stage === 0) {
        if (buf.length < 2) return
        const nmethods = buf[1] ?? 0
        if (buf.length < 2 + nmethods) return
        buf = buf.subarray(2 + nmethods)
        socket.write(Buffer.from([0x05, 0x00]))
        stage = 1
      }
      if (stage !== 1) return
      if (buf.length < 4) return
      const atyp = buf[3]
      let host = DEST
      let offset = 4
      if (atyp === 1) {
        if (buf.length < 10) return
        host = Array.from(buf.subarray(4, 8)).join('.')
        offset = 8
      } else if (atyp === 3) {
        if (buf.length < 5) return
        const len = buf[4] ?? 0
        if (buf.length < 5 + len + 2) return
        host = buf.subarray(5, 5 + len).toString()
        offset = 5 + len
      } else if (atyp === 4) {
        if (buf.length < 22) return
        offset = 20
      } else {
        socket.end()
        return
      }
      const port = buf.readUInt16BE(offset)
      const rest = buf.subarray(offset + 2)
      stage = 2
      socket.off('data', onData)
      hit.n += 1
      const target = /^\d+\.\d+\.\d+\.\d+$/.test(host) ? host : DEST
      const upstream = net.connect(port, target)
      upstream.on('connect', () => {
        socket.write(Buffer.from([0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0]))
        if (rest.length) upstream.write(rest)
        socket.pipe(upstream)
        upstream.pipe(socket)
      })
      upstream.on('error', () => socket.destroy())
      socket.on('error', () => upstream.destroy())
    }
    socket.on('data', onData)
    socket.on('error', () => {})
  })
  return new Promise<net.Server>((resolve) => {
    server.listen(SOCKS_PORT, '127.0.0.1', () => resolve(server))
  })
}

const liveConfig = (mode: 'rule' | 'global', pinnedRoutes: PinnedRoutes) => {
  const base = sampleBaseConfig()
  base.route.rules = base.route.rules.filter((rule) => rule.rule_set !== 'geoip-private')
  base.route.rule_set = [{ type: 'inline', tag: 'geosite-cn', rules: [{ domain_suffix: ['cn'] }] }]
  base.experimental = {
    clash_api: { external_controller: '127.0.0.1:9090', default_mode: mode },
  }
  const config = applyAppRouting(base, {
    programs: [
      { id: 'chrome', name: 'Google Chrome', exe: 'chrome.exe', mode: 'proxy' },
      { id: 'edge', name: 'Microsoft Edge', exe: 'msedge.exe', mode: 'direct' },
    ],
    extraDirectExes: ['douyin_extra.exe'],
    pinnedRoutes,
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
  })
  config.inbounds = (config.inbounds || []).filter((inbound: { type?: string }) => inbound.type !== 'tun')
  const mixed = (config.inbounds as { type?: string; listen?: string; listen_port?: number }[]).find(
    (inbound) => inbound.type === 'mixed',
  )
  if (mixed) {
    mixed.listen = '127.0.0.1'
    mixed.listen_port = MIXED_PORT
  }
  const socks = (config.outbounds as Record<string, unknown>[]).find((outbound) => outbound.tag === 'anytls-hk')
  if (socks) {
    delete socks.password
    delete socks.tls
    socks.type = 'socks'
    socks.server = '127.0.0.1'
    socks.server_port = SOCKS_PORT
    socks.version = '5'
  }
  const hosts = {
    'weixin.qq.com': [DEST],
    'qq.com': [DEST],
    'douyin.com': [DEST],
    'example.com': [DEST],
  }
  for (const server of config.dns.servers as Record<string, unknown>[]) {
    if (server.tag === 'dhagn-local-dns' || server.tag === 'Remote-DNS' || server.tag === 'Local-DNS') {
      for (const key of Object.keys(server)) {
        if (key !== 'tag') delete server[key]
      }
      server.type = 'hosts'
      server.predefined = hosts
    }
  }
  config.route.default_domain_resolver = { server: 'dhagn-local-dns', strategy: 'ipv4_only' }
  config.log = { level: 'info' }
  return config
}

const waitForPort = (port: number, child: ChildProcess) =>
  new Promise<void>((resolve, reject) => {
    const started = Date.now()
    const tick = () => {
      if (child.exitCode !== null) {
        reject(new Error('sing-box exited before the mixed inbound was ready'))
        return
      }
      const socket = net.connect(port, '127.0.0.1')
      socket.on('connect', () => {
        socket.end()
        resolve()
      })
      socket.on('error', () => {
        socket.destroy()
        if (Date.now() - started > 8000) reject(new Error(`timed out waiting for 127.0.0.1:${port}`))
        else setTimeout(tick, 100)
      })
    }
    tick()
  })

const runCurl = (bin: string, url: string, hit: { n: number }) =>
  new Promise<{ status: number | null; stdout: string; stderr: string; socks: number }>((resolve) => {
    const before = hit.n
    const child = spawn(
      bin,
      ['-4', '-sS', '--http1.1', '--retry', '0', '--max-time', '5', '-x', `http://127.0.0.1:${MIXED_PORT}`, '-o', '/dev/null', '-w', '%{http_code}', url],
      {
        env: {
          ...process.env,
          http_proxy: '',
          https_proxy: '',
          HTTP_PROXY: '',
          HTTPS_PROXY: '',
          ALL_PROXY: '',
          all_proxy: '',
        },
      },
    )
    let stdout = ''
    let stderr = ''
    child.stdout.on('data', (chunk) => {
      stdout += chunk
    })
    child.stderr.on('data', (chunk) => {
      stderr += chunk
    })
    child.on('close', (status) => {
      resolve({ status, stdout, stderr, socks: hit.n - before })
    })
  })

const withSingBox = async (config: Record<string, unknown>, fn: (logs: () => string) => Promise<void>) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dhagn-netns-'))
  const file = path.join(dir, 'config.json')
  fs.writeFileSync(file, JSON.stringify(config))
  const check = spawnSync('sing-box', ['check', '-c', file], { encoding: 'utf8' })
  if (check.status !== 0) {
    throw new Error(`sing-box check failed\n${check.stderr || check.stdout}`)
  }
  const child = spawn('sing-box', ['run', '-c', file], { stdio: ['ignore', 'pipe', 'pipe'] })
  let logs = ''
  child.stdout?.on('data', (chunk) => {
    logs += chunk
  })
  child.stderr?.on('data', (chunk) => {
    logs += chunk
  })
  try {
    await waitForPort(MIXED_PORT, child)
    await fn(() => logs)
  } finally {
    child.kill('SIGTERM')
    await new Promise((resolve) => child.on('exit', resolve))
  }
}

const cases: {
  name: string
  mode: 'rule' | 'global'
  pinned: PinnedRoutes
  requests: { bin: string; url: string; socks: boolean }[]
}[] = [
  {
    name: 'both direct in rule mode',
    mode: 'rule',
    pinned: { wechat: 'direct', douyin: 'direct' },
    requests: [
      { bin: 'Weixin.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: false },
      { bin: 'douyin.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: false },
      { bin: 'douyin_extra.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: false },
      { bin: 'notepad.exe', url: `http://weixin.qq.com:${HTTP_PORT}/`, socks: false },
      { bin: 'notepad.exe', url: `http://douyin.com:${HTTP_PORT}/`, socks: false },
      { bin: 'chrome.exe', url: `http://example.com:${HTTP_PORT}/`, socks: true },
    ],
  },
  {
    name: 'both proxy in rule mode',
    mode: 'rule',
    pinned: { wechat: 'proxy', douyin: 'proxy' },
    requests: [
      { bin: 'Weixin.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: true },
      { bin: 'douyin.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: true },
      { bin: 'douyin_extra.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: true },
      { bin: 'notepad.exe', url: `http://weixin.qq.com:${HTTP_PORT}/`, socks: true },
      { bin: 'notepad.exe', url: `http://qq.com:${HTTP_PORT}/`, socks: true },
      { bin: 'notepad.exe', url: `http://douyin.com:${HTTP_PORT}/`, socks: true },
      { bin: 'notepad.exe', url: `http://example.com:${HTTP_PORT}/`, socks: false },
    ],
  },
  {
    name: 'WeChat proxy and Douyin direct in global mode',
    mode: 'global',
    pinned: { wechat: 'proxy', douyin: 'direct' },
    requests: [
      { bin: 'Weixin.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: true },
      { bin: 'douyin.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: false },
      { bin: 'notepad.exe', url: `http://weixin.qq.com:${HTTP_PORT}/`, socks: true },
      { bin: 'notepad.exe', url: `http://douyin.com:${HTTP_PORT}/`, socks: false },
      { bin: 'chrome.exe', url: `http://example.com:${HTTP_PORT}/`, socks: true },
      { bin: 'notepad.exe', url: `http://example.com:${HTTP_PORT}/`, socks: false },
    ],
  },
  {
    name: 'both direct in global mode stays direct',
    mode: 'global',
    pinned: { wechat: 'direct', douyin: 'direct' },
    requests: [
      { bin: 'Weixin.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: false },
      { bin: 'douyin.exe', url: `http://${DEST}:${HTTP_PORT}/`, socks: false },
      { bin: 'notepad.exe', url: `http://weixin.qq.com:${HTTP_PORT}/`, socks: false },
      { bin: 'notepad.exe', url: `http://douyin.com:${HTTP_PORT}/`, socks: false },
    ],
  },
]

const runNetns = async () => {
  prepareNet()
  const binDir = fs.mkdtempSync(path.join(os.tmpdir(), 'dhagn-bins-'))
  for (const name of ['Weixin.exe', 'douyin.exe', 'douyin_extra.exe', 'notepad.exe', 'chrome.exe']) {
    const dest = path.join(binDir, name)
    fs.copyFileSync('/usr/bin/curl', dest)
    fs.chmodSync(dest, 0o755)
  }
  const hit = { n: 0 }
  const httpServer = http.createServer((_req, res) => {
    res.writeHead(200)
    res.end('ok')
  })
  await new Promise<void>((resolve) => httpServer.listen(HTTP_PORT, DEST, () => resolve()))
  const socks = await startSocks(hit)
  try {
    for (const item of cases) {
      await withSingBox(liveConfig(item.mode, item.pinned), async (logs) => {
        for (const request of item.requests) {
          const result = await runCurl(path.join(binDir, request.bin), request.url, hit)
          const wentProxy = result.socks > 0
          if (result.status !== 0 || result.stdout !== '200' || wentProxy !== request.socks) {
            throw new Error(
              [
                item.name,
                `${request.bin} ${request.url}`,
                `exit=${result.status} http=${result.stdout || '-'} socks=${result.socks} expectedSocks=${request.socks}`,
                result.stderr.trim(),
                logs().trim().split('\n').slice(-40).join('\n'),
              ]
                .filter(Boolean)
                .join('\n'),
            )
          }
        }
      })
    }
    process.stdout.write('netns routing ok\n')
  } finally {
    await new Promise((resolve) => httpServer.close(resolve))
    await new Promise((resolve) => socks.close(resolve))
  }
}

if (process.env.DHAGN_NETNS === '1') {
  await runNetns()
} else {
  test('netns sing-box routes WeChat and Douyin by the saved choice', { timeout: 90_000 }, async (t) => {
    const probe = spawnSync('unshare', ['-n', '-r', 'true'], { encoding: 'utf8' })
    if (probe.status !== 0) {
      t.skip('unshare -n -r is not available')
      return
    }
    const result = spawnSync(
      'unshare',
      ['-n', '-r', process.execPath, '--experimental-strip-types', fileURLToPath(import.meta.url)],
      {
        env: { ...process.env, DHAGN_NETNS: '1' },
        encoding: 'utf8',
        timeout: 80_000,
      },
    )
    if (result.status !== 0) {
      throw new Error([result.stdout, result.stderr].filter(Boolean).join('\n') || `inner exit ${result.status}`)
    }
  })
}
