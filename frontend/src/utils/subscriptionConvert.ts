/**
 * Turn airport subscription payloads into sing-box outbounds.
 * sing-box JSON outbounds pass through. Clash proxies and share links,
 * especially anytls (sing-box ≥ 1.12), are converted in-app so the 订阅 page
 * does not depend on the upstream 节点转换 plugin.
 */

export interface SingboxOutbound {
  type: string
  tag: string
  [key: string]: unknown
}

const asString = (value: unknown) => (value == null ? '' : String(value))

export const decodeBase64Text = (input: string) => {
  const normalized = input.replace(/-/g, '+').replace(/_/g, '/').replace(/\s+/g, '')
  const pad = normalized.length % 4 === 0 ? '' : '='.repeat(4 - (normalized.length % 4))
  const binary = atob(normalized + pad)
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

const tlsFromClash = (proxy: Record<string, any>, server: string, force = false) => {
  const enabled =
    force ||
    proxy.tls === true ||
    proxy.tls === 'true' ||
    proxy.tls === 'tls' ||
    !!proxy.sni ||
    !!proxy.servername
  if (!enabled) return undefined
  const tls: Record<string, unknown> = {
    enabled: true,
    server_name: proxy.sni || proxy.servername || server,
  }
  if (proxy['skip-cert-verify'] || proxy.skip_cert_verify || proxy.insecure) {
    tls.insecure = true
  }
  if (proxy.alpn) tls.alpn = Array.isArray(proxy.alpn) ? proxy.alpn : [proxy.alpn]
  const fingerprint = proxy['client-fingerprint'] || proxy.client_fingerprint || proxy.fingerprint
  if (fingerprint) tls.utls = { enabled: true, fingerprint }
  return tls
}

const anytlsOutbound = (
  tag: string,
  server: string,
  port: number,
  password: string,
  tls: Record<string, unknown>,
): SingboxOutbound => ({
  type: 'anytls',
  tag,
  server,
  server_port: port,
  password,
  tls,
})

export const clashProxyToOutbound = (proxy: Record<string, any>): SingboxOutbound | undefined => {
  const tag = asString(proxy.name || proxy.tag).trim()
  const type = asString(proxy.type).trim().toLowerCase()
  const server = asString(proxy.server).trim()
  const port = Number(proxy.port || proxy.server_port || 443)
  if (!tag || !type || !server) return undefined

  if (type === 'anytls') {
    return anytlsOutbound(
      tag,
      server,
      port,
      asString(proxy.password),
      tlsFromClash(proxy, server, true)!,
    )
  }

  if (type === 'ss' || type === 'shadowsocks') {
    return {
      type: 'shadowsocks',
      tag,
      server,
      server_port: port,
      method: proxy.cipher || proxy.method,
      password: asString(proxy.password),
    }
  }

  if (type === 'trojan') {
    return {
      type: 'trojan',
      tag,
      server,
      server_port: port,
      password: asString(proxy.password),
      tls: tlsFromClash(proxy, server, true),
    }
  }

  if (type === 'vless') {
    return {
      type: 'vless',
      tag,
      server,
      server_port: port,
      uuid: proxy.uuid,
      tls: tlsFromClash(proxy, server),
    }
  }

  if (type === 'vmess') {
    return {
      type: 'vmess',
      tag,
      server,
      server_port: port,
      uuid: proxy.uuid,
      security: proxy.cipher || proxy.security || 'auto',
      alter_id: Number(proxy.alterId ?? proxy.alter_id ?? 0),
      tls: tlsFromClash(proxy, server),
    }
  }

  if (type === 'hysteria2' || type === 'hy2') {
    return {
      type: 'hysteria2',
      tag,
      server,
      server_port: port,
      password: asString(proxy.password),
      tls: tlsFromClash(proxy, server, true),
    }
  }

  if (type === 'tuic') {
    return {
      type: 'tuic',
      tag,
      server,
      server_port: port,
      uuid: proxy.uuid,
      password: asString(proxy.password),
      tls: tlsFromClash(proxy, server, true),
    }
  }

  return undefined
}

const parseShareLine = (line: string): SingboxOutbound | undefined => {
  const text = line.trim()
  if (!text || text.startsWith('#') || text.startsWith('//')) return undefined
  if (!/^[a-z0-9+.-]+:\/\//i.test(text)) return undefined

  let url: URL
  try {
    url = new URL(text)
  } catch {
    return undefined
  }
  const scheme = url.protocol.replace(':', '').toLowerCase()
  const tag = decodeURIComponent(url.hash.replace(/^#/, '')) || `${url.hostname}:${url.port}`
  const server = url.hostname
  const port = Number(url.port || (scheme === 'anytls' ? 443 : 443))
  if (!server) return undefined

  if (scheme === 'anytls') {
    const tls: Record<string, unknown> = {
      enabled: true,
      server_name: url.searchParams.get('sni') || url.searchParams.get('peer') || server,
    }
    const insecure = url.searchParams.get('insecure') || url.searchParams.get('allowInsecure')
    if (insecure === '1' || insecure === 'true') tls.insecure = true
    const alpn = url.searchParams.get('alpn')
    if (alpn) tls.alpn = alpn.split(',')
    const fingerprint = url.searchParams.get('fp') || url.searchParams.get('fingerprint')
    if (fingerprint) tls.utls = { enabled: true, fingerprint }
    return anytlsOutbound(tag, server, port, decodeURIComponent(url.username), tls)
  }

  if (scheme === 'trojan') {
    return {
      type: 'trojan',
      tag,
      server,
      server_port: port,
      password: decodeURIComponent(url.username),
      tls: {
        enabled: true,
        server_name: url.searchParams.get('sni') || server,
        insecure: url.searchParams.get('allowInsecure') === '1',
      },
    }
  }

  if (scheme === 'vless') {
    return {
      type: 'vless',
      tag,
      server,
      server_port: port,
      uuid: decodeURIComponent(url.username),
      tls:
        url.searchParams.get('security') === 'tls'
          ? {
              enabled: true,
              server_name: url.searchParams.get('sni') || server,
            }
          : undefined,
    }
  }

  if (scheme === 'vmess') return parseVmessLink(text)
  if (scheme === 'ss') return parseShadowsocksLink(text)

  return undefined
}

const parseVmessLink = (text: string): SingboxOutbound | undefined => {
  const payload = text.slice('vmess://'.length).split('#')[0] || ''
  let info: Record<string, any>
  try {
    info = JSON.parse(decodeBase64Text(payload))
  } catch {
    return undefined
  }
  const server = asString(info.add || info.addr || info.server).trim()
  const port = Number(info.port || 443)
  if (!server || !port) return undefined
  const tag = asString(info.ps || info.remark).trim() || `${server}:${port}`
  const tlsOn = info.tls === 'tls' || info.tls === true || info.tls === '1'
  const outbound: SingboxOutbound = {
    type: 'vmess',
    tag,
    server,
    server_port: port,
    uuid: asString(info.id || info.uuid),
    security: asString(info.scy || info.security || 'auto') || 'auto',
  }
  if (tlsOn) {
    outbound.tls = { enabled: true, server_name: asString(info.sni || info.host || server) || server }
  }
  if (asString(info.net).toLowerCase() === 'ws') {
    const host = asString(info.host).trim()
    outbound.transport = {
      type: 'ws',
      path: asString(info.path || '/'),
      ...(host ? { headers: { Host: host } } : {}),
    }
  }
  return outbound
}

const parseShadowsocksLink = (text: string): SingboxOutbound | undefined => {
  const hashAt = text.indexOf('#')
  const tag = hashAt >= 0 ? decodeURIComponent(text.slice(hashAt + 1)) : ''
  const body = (hashAt >= 0 ? text.slice('ss://'.length, hashAt) : text.slice('ss://'.length)).trim()
  const toOutbound = (method: string, password: string, server: string, port: number, name: string) => {
    if (!method || !server || !port) return undefined
    return {
      type: 'shadowsocks',
      tag: name || `${server}:${port}`,
      server,
      server_port: port,
      method,
      password,
    } satisfies SingboxOutbound
  }
  const at = body.lastIndexOf('@')
  if (at > 0) {
    let user = body.slice(0, at)
    const hostport = body.slice(at + 1)
    try {
      if (!user.includes(':')) user = decodeBase64Text(user)
    } catch {
      return undefined
    }
    const split = user.indexOf(':')
    const colon = hostport.lastIndexOf(':')
    if (split <= 0 || colon <= 0) return undefined
    return toOutbound(
      user.slice(0, split),
      user.slice(split + 1),
      hostport.slice(0, colon),
      Number(hostport.slice(colon + 1)),
      tag,
    )
  }
  try {
    const decoded = decodeBase64Text(body)
    const atDecoded = decoded.lastIndexOf('@')
    const split = decoded.indexOf(':')
    if (atDecoded <= split || split <= 0) return undefined
    const hostport = decoded.slice(atDecoded + 1)
    const colon = hostport.lastIndexOf(':')
    if (colon <= 0) return undefined
    return toOutbound(
      decoded.slice(0, split),
      decoded.slice(split + 1, atDecoded),
      hostport.slice(0, colon),
      Number(hostport.slice(colon + 1)),
      tag,
    )
  } catch {
    return undefined
  }
}

export const parseShareText = (text: string): SingboxOutbound[] => {
  const trimmed = text.trim()
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      const parsed = JSON.parse(trimmed)
      const list = Array.isArray(parsed) ? parsed : parsed.outbounds
      if (Array.isArray(list)) return normalizeSubscriptionProxies(list)
    } catch {
      // fall through to line parsing
    }
  }
  return trimmed
    .split(/\r?\n/)
    .map((line) => parseShareLine(line))
    .filter((item): item is SingboxOutbound => !!item)
}

const isSingboxOutbound = (item: Record<string, any>) =>
  !!item.tag && !!item.type && !item.name && (item.server || item.server_port || item.outbounds || item.password || item.uuid)

export const normalizeSubscriptionProxies = (proxies: Record<string, any>[]): SingboxOutbound[] => {
  const result: SingboxOutbound[] = []
  for (const item of proxies || []) {
    if (!item || typeof item !== 'object') continue
    if (typeof item.base64 === 'string') {
      try {
        result.push(...parseShareText(decodeBase64Text(item.base64)))
      } catch {
        // ignore a broken share blob and keep converting the rest
      }
      continue
    }
    if (isSingboxOutbound(item)) {
      result.push(item as SingboxOutbound)
      continue
    }
    if (item.name && item.type) {
      const converted = clashProxyToOutbound(item)
      if (converted) result.push(converted)
    }
  }
  return result
}
