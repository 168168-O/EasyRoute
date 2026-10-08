export interface CountryGroup {
  id: string
  tag: string
  keywords: string[]
}

export const COUNTRY_GROUPS: CountryGroup[] = [
  { id: 'hk', tag: '🇭🇰 香港', keywords: ['香港', 'hong kong', 'hongkong'] },
  { id: 'jp', tag: '🇯🇵 日本', keywords: ['日本', 'japan', 'tokyo', 'osaka'] },
  { id: 'sg', tag: '🇸🇬 新加坡', keywords: ['新加坡', 'singapore'] },
  { id: 'tw', tag: '🇹🇼 台湾', keywords: ['台湾', '台灣', 'taiwan'] },
  { id: 'us', tag: '🇺🇸 美国', keywords: ['美国', '美國', 'united states', 'america'] },
  { id: 'other', tag: '🌐 其他', keywords: [] },
]

const LATIN = [
  { id: 'hk', pattern: /(^|[^a-z])hk([^a-z]|$)/ },
  { id: 'jp', pattern: /(^|[^a-z])jp([^a-z]|$)/ },
  { id: 'sg', pattern: /(^|[^a-z])sg([^a-z]|$)/ },
  { id: 'tw', pattern: /(^|[^a-z])tw([^a-z]|$)/ },
  { id: 'us', pattern: /(^|[^a-z])(us|usa)([^a-z]|$)/ },
]

export const countryGroupTags = new Set(COUNTRY_GROUPS.map((group) => group.tag))

/** First matching region. Latin codes use a boundary so NHK does not become Hong Kong. */
export const classifyNode = (tag: string) => {
  const value = tag.toLowerCase()
  for (const group of COUNTRY_GROUPS) {
    if (group.keywords.some((keyword) => value.includes(keyword))) return group.id
  }
  for (const item of LATIN) {
    if (item.pattern.test(value)) return item.id
  }
  return 'other'
}

export interface UrlTestGroup {
  type: 'urltest'
  tag: string
  outbounds: string[]
  url: string
  interval: string
  tolerance: number
}

export const buildCountryUrltests = (
  tags: string[],
  options: { url?: string; interval?: string } = {},
): UrlTestGroup[] => {
  const buckets = new Map<string, string[]>()
  for (const tag of tags) {
    const name = tag.trim()
    if (!name || countryGroupTags.has(name)) continue
    const id = classifyNode(name)
    const list = buckets.get(id) || []
    if (!list.includes(name)) list.push(name)
    buckets.set(id, list)
  }
  return COUNTRY_GROUPS.flatMap((group) => {
    const outbounds = buckets.get(group.id) || []
    if (!outbounds.length) return []
    return [
      {
        type: 'urltest' as const,
        tag: group.tag,
        outbounds,
        url: options.url || 'https://www.gstatic.com/generate_204',
        interval: options.interval || '10m',
        tolerance: 50,
      },
    ]
  })
}

interface GeneratedOutbound {
  type?: string
  tag?: string
  outbounds?: string[]
  url?: string
  interval?: string
  tolerance?: number
}

/** Put region urltest groups at the front of 节点选择. Leaf nodes stay in place. */
export const attachCountryGroups = <T extends GeneratedOutbound>(outbounds: T[], testUrl = ''): T[] => {
  const select = outbounds.find((item) => item.type === 'selector' && String(item.tag || '').includes('节点选择'))
  if (!select || !Array.isArray(select.outbounds)) return outbounds
  const groupTags = new Set(
    outbounds
      .filter((item) => item.type === 'selector' || item.type === 'urltest')
      .map((item) => String(item.tag || '')),
  )
  const nested = outbounds.find(
    (item) => item.type === 'urltest' && select.outbounds?.includes(String(item.tag || '')),
  )
  const leaves = [
    ...select.outbounds.filter((tag) => tag && !groupTags.has(tag) && tag !== 'direct' && tag !== 'block'),
    ...((nested?.outbounds || []).filter((tag) => tag && !groupTags.has(tag))),
  ]
  const groups = buildCountryUrltests(leaves, { url: testUrl || undefined, interval: '10m' })
  if (!groups.length) return outbounds.filter((item) => !countryGroupTags.has(String(item.tag || ''))) as T[]
  const next = outbounds.filter((item) => !countryGroupTags.has(String(item.tag || '')))
  const target = next.find((item) => item === select) || next.find((item) => item.type === 'selector' && String(item.tag || '').includes('节点选择'))
  if (!target || !Array.isArray(target.outbounds)) return next as T[]
  target.outbounds = [
    ...groups.map((group) => group.tag),
    ...target.outbounds.filter((tag) => !countryGroupTags.has(tag)),
  ]
  return [...next, ...(groups as unknown as T[])]
}
