import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'

import { applyAppRouting, sampleBaseConfig } from '../src/utils/appRouting.ts'

const programs = [
  { id: 'chrome', name: 'Google Chrome', exe: 'chrome.exe', mode: 'proxy' as const },
  { id: 'edge', name: 'Microsoft Edge', exe: 'msedge.exe', mode: 'direct' as const },
]

const checkedConfig = (
  mode: 'rule' | 'global',
  pinnedRoutes?: { wechat: 'proxy' | 'direct'; douyin: 'proxy' | 'direct' },
) => {
  const base = sampleBaseConfig()
  base.inbounds.push({ type: 'http', tag: 'http-in', listen: '127.0.0.1', listen_port: 20123 })
  base.dns.servers.push({ type: 'fakeip', tag: 'fakeip-dns', inet4_range: '198.18.0.0/15' })
  const anytls = base.outbounds.find((outbound) => outbound.type === 'anytls')
  if (anytls) anytls.tls = { enabled: true, server_name: 'example.invalid' }
  base.route.rules = base.route.rules.filter((rule) => rule.rule_set !== 'geoip-private')
  base.route.rule_set = [
    { type: 'inline', tag: 'geosite-cn', rules: [{ domain_suffix: ['cn'] }] },
  ]
  base.experimental = {
    clash_api: {
      external_controller: '127.0.0.1:9090',
      default_mode: mode,
    },
  }
  return applyAppRouting(base, {
    programs,
    extraDirectExes: ['douyin_extra.exe'],
    pinnedRoutes,
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
    appPath: String.raw`C:\达货爱vpn姑娘\达货爱vpn姑娘.exe`,
  })
}

const singBox = () => {
  try {
    execFileSync('sing-box', ['version'], { stdio: 'pipe' })
    return 'sing-box'
  } catch {
    return ''
  }
}

test('sing-box check accepts the generated config in rule and global mode', () => {
  const bin = singBox()
  assert.ok(bin, 'sing-box 1.14.2 must be on PATH')
  const dir = mkdtempSync(join(tmpdir(), 'dhagn-singbox-'))
  const pinnedStates = [
    undefined,
    { wechat: 'direct' as const, douyin: 'direct' as const },
    { wechat: 'proxy' as const, douyin: 'proxy' as const },
    { wechat: 'proxy' as const, douyin: 'direct' as const },
    { wechat: 'direct' as const, douyin: 'proxy' as const },
  ]
  for (const mode of ['rule', 'global'] as const) {
    for (const pinnedRoutes of pinnedStates) {
      const label = `${mode}-${pinnedRoutes?.wechat ?? 'default'}-${pinnedRoutes?.douyin ?? 'default'}`
      const file = join(dir, `config-${label}.json`)
      writeFileSync(file, JSON.stringify(checkedConfig(mode, pinnedRoutes)))
      execFileSync(bin, ['check', '-c', file], { stdio: 'pipe' })
    }
  }
  const version = execFileSync(bin, ['version'], { encoding: 'utf8' })
  assert.match(version, /1\.14\.2/)
})
