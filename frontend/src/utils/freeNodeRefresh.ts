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
  parseFreeSource,
  rankFreeNodes,
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

/** Pull the configured sources, test slowly, and store at most five working nodes. */
export const refreshFreeBackup = async () => {
  const store = useSubscribesStore()
  const sub = store.getSubscribeById(FREE_NODE_SUB_ID)
  if (!sub || sub.disabled) return
  if (!dueForFreeRefresh(sub.updateTime)) return

  const previous = await readStored(sub.path)
  const bodies: string[] = []
  for (const url of freeNodeSources(sub)) {
    try {
      bodies.push(await fetchSource(url))
    } catch {
      // The next source may still answer. A total miss keeps the old list.
    }
  }

  let best: SingboxOutbound[] = []
  if (bodies.length) {
    const candidates = collectFreeCandidates(bodies.map((body) => parseFreeSource(body)))
    if (candidates.length) {
      const probed: { node: SingboxOutbound; ok: boolean; delayMs: number }[] = []
      await asyncPool(FREE_PROBE_CONCURRENCY, candidates, async (node) => {
        const result = await probeFreeNode(node)
        probed.push({ node, ...result })
      })
      best = rankFreeNodes(probed, FREE_KEEP)
    }
  }

  const decision = decideFreeList(previous, best)
  if (decision.replace) {
    await WriteFile(sub.path, JSON.stringify(decision.proxies, null, 2))
    sub.proxies = decision.proxies.map((node) => ({
      id: sub.proxies.find((item) => item.tag === node.tag)?.id || sampleID(),
      tag: node.tag,
      type: node.type,
    }))
  }
  sub.updateTime = Date.now()
  await store.saveSubscribes()
}
