import { ReadFile, Requests, WriteFile } from '@/bridge'
import { RequestProxyMode } from '@/enums/app'
import { useSubscribesStore } from '@/stores'
import { GetRequestProxy, asyncPool, sampleID } from '@/utils'

import {
  FREE_KEEP,
  FREE_NODE_SUB_ID,
  FREE_PROBE_CONCURRENCY,
  collectFreeCandidates,
  decideFreeList,
  dueForFreeRefresh,
  freeNodeSources,
  isProxyOutbound,
  parseFreeSource,
} from './freeNodes.ts'
import { probeFreeNode } from './freeNodeProbe.ts'
import type { SingboxOutbound } from './subscriptionConvert.ts'

const readStored = async (path: string): Promise<SingboxOutbound[]> => {
  try {
    const text = await ReadFile(path)
    const parsed = JSON.parse(text) as unknown
    return Array.isArray(parsed) ? (parsed as SingboxOutbound[]) : []
  } catch {
    return []
  }
}

const fetchSource = async (url: string) => {
  const once = async (proxy: string) => {
    const response = await Requests({
      method: 'GET',
      url,
      headers: { 'User-Agent': 'clash.meta/mihomo' },
      autoTransformBody: false,
      options: { Proxy: proxy, Timeout: 20 },
    })
    const body = typeof response.body === 'string' ? response.body : JSON.stringify(response.body ?? '')
    if (!body.trim() || response.status >= 400) throw new Error('empty')
    return body
  }
  try {
    return await once('direct')
  } catch {
    const kernel = await GetRequestProxy(RequestProxyMode.Kernel).catch(() => '')
    if (!kernel) throw new Error('unavailable')
    return await once(kernel)
  }
}

const remember = (sub: App.Subscription, proxies: SingboxOutbound[]) => {
  sub.proxies = proxies.map((node) => ({
    id: sub.proxies.find((item) => item.tag === node.tag)?.id || sampleID(),
    tag: node.tag,
    type: node.type,
  }))
}

/**
 * Retest the stored nodes plus a fresh sample.
 * Only the five fastest working nodes are written back. A node that died or
 * is too slow is removed, and a working candidate takes its place.
 * Returns true when the stored list changed.
 */
export const refreshFreeBackup = async () => {
  const store = useSubscribesStore()
  const sub = store.getSubscribeById(FREE_NODE_SUB_ID)
  if (!sub || sub.disabled) return false
  if (!dueForFreeRefresh(sub.updateTime)) return false

  const raw = await readStored(sub.path)
  const previous = raw.filter(isProxyOutbound).slice(0, FREE_KEEP)
  const batches: SingboxOutbound[][] = [previous]
  for (const url of freeNodeSources(sub)) {
    try {
      batches.push(parseFreeSource(await fetchSource(url)))
    } catch {
      // Another source, or the stored nodes, can still be tested.
    }
  }

  const candidates = collectFreeCandidates(batches)
  const hadExtras = raw.length !== previous.length || sub.proxies.length !== previous.length
  if (!candidates.length) {
    if (hadExtras || sub.proxies.length) {
      await WriteFile(sub.path, '[]')
      sub.proxies = []
      sub.updateTime = Date.now()
      await store.saveSubscribes()
      return true
    }
    sub.updateTime = Date.now()
    await store.saveSubscribes()
    return false
  }

  const probed: { node: SingboxOutbound; ok: boolean; delayMs: number; unavailable?: boolean }[] = []
  await asyncPool(FREE_PROBE_CONCURRENCY, candidates, async (node) => {
    const result = await probeFreeNode(node)
    probed.push({ node, ...result })
  })

  const decision = decideFreeList(previous, probed)
  if (!decision.replace) {
    if (!hadExtras) return false
    await WriteFile(sub.path, JSON.stringify(previous, null, 2))
    remember(sub, previous)
    await store.saveSubscribes()
    return true
  }

  const same =
    !hadExtras &&
    decision.proxies.length === previous.length &&
    decision.proxies.every((node, index) => node.tag === previous[index]?.tag) &&
    sub.proxies.length === decision.proxies.length &&
    sub.proxies.every((item, index) => item.tag === decision.proxies[index]?.tag)
  await WriteFile(sub.path, JSON.stringify(decision.proxies, null, 2))
  remember(sub, decision.proxies)
  sub.updateTime = Date.now()
  await store.saveSubscribes()
  return !same
}
