export type RedeemNoticePart = {
  text: string
  href?: string
}

const HTTP_URL = /https?:\/\/[^\s<>"'，。！？；：）】》、]+/giu
const TRAILING_PUNCTUATION = /[.,!?;:)\]}，。！？；：）】》、]+$/u

export function splitRedeemNoticeLinks(notice: string): RedeemNoticePart[] {
  const parts: RedeemNoticePart[] = []
  let cursor = 0

  for (const match of notice.matchAll(HTTP_URL)) {
    const start = match.index ?? 0
    const raw = match[0]
    let href = raw
    const trailing = href.match(TRAILING_PUNCTUATION)?.[0] ?? ''
    if (trailing) href = href.slice(0, -trailing.length)

    try {
      const parsed = new URL(href)
      if ((parsed.protocol !== 'http:' && parsed.protocol !== 'https:') || !parsed.hostname) continue
    } catch {
      continue
    }

    if (start > cursor) parts.push({ text: notice.slice(cursor, start) })
    parts.push({ text: href, href })
    if (trailing) parts.push({ text: trailing })
    cursor = start + raw.length
  }

  if (cursor < notice.length) parts.push({ text: notice.slice(cursor) })
  return parts.length ? parts : [{ text: notice }]
}
