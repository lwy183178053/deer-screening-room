export type StudioLayoutItem = {
  id: number
  width: number
  order?: number
}

type LayoutRow = {
  items: StudioLayoutItem[]
  width: number
}

function layoutKey(rows: LayoutRow[]) {
  return rows.flatMap(row => row.items.map(item => String(item.order ?? 0).padStart(6, '0'))).join(',')
}

function isBetter(candidate: LayoutRow[], current: LayoutRow[] | null, containerWidth: number) {
  if (!current) return true
  if (candidate.length !== current.length) return candidate.length < current.length
  const candidateSlack = candidate.reduce((total, row) => total + (containerWidth - row.width) ** 2, 0)
  const currentSlack = current.reduce((total, row) => total + (containerWidth - row.width) ** 2, 0)
  if (candidateSlack !== currentSlack) return candidateSlack < currentSlack
  return layoutKey(candidate) < layoutKey(current)
}

export function packStudioRows(items: readonly StudioLayoutItem[], containerWidth: number, gap = 8): number[][] {
  if (!items.length) return []
  if (containerWidth <= 0) return [items.map(item => item.id)]

  const rowGap = Math.max(gap, 0)
  const normalized = items.map((item, order) => ({ ...item, order: item.order ?? order, width: Math.min(Math.max(item.width, 0), containerWidth) }))
  const recommendation = normalized.find(item => item.id === 0)
  const remaining = normalized.filter(item => item.id !== 0).sort((a, b) => b.width - a.width || (a.order ?? 0) - (b.order ?? 0))
  const greedyRows: LayoutRow[] = recommendation ? [{ items: [recommendation], width: recommendation.width }] : []
  for (const item of remaining) {
    let target: LayoutRow | undefined
    let targetWidth = -Infinity
    for (const row of greedyRows) {
      const nextWidth = row.items.length ? row.width + rowGap + item.width : item.width
      if (nextWidth > containerWidth) continue
      if (!target || nextWidth > targetWidth) { target = row; targetWidth = nextWidth }
    }
    if (target) { target.items.push(item); target.width = targetWidth }
    else greedyRows.push({ items: [item], width: item.width })
  }
  let best: LayoutRow[] | null = greedyRows.map(row => ({ items: [...row.items], width: row.width }))
  const deadline = Date.now() + 16
  let timedOut = false

  const search = (index: number, rows: LayoutRow[]) => {
    if (timedOut || Date.now() >= deadline) {
      timedOut = true
      return
    }
    if (best && rows.length > best.length) return
    if (index >= remaining.length) {
      const candidate = rows.map(row => ({ items: [...row.items], width: row.width }))
      if (isBetter(candidate, best, containerWidth)) best = candidate
      return
    }

    const item = remaining[index]
    const seenWidths = new Set<number>()
    for (const row of rows) {
      if (seenWidths.has(row.width)) continue
      seenWidths.add(row.width)
      const nextWidth = row.items.length ? row.width + rowGap + item.width : item.width
      if (nextWidth > containerWidth) continue
      row.items.push(item)
      const previousWidth = row.width
      row.width = nextWidth
      search(index + 1, rows)
      row.width = previousWidth
      row.items.pop()
    }

    if (!best || rows.length + 1 <= best.length) search(index + 1, [...rows, { items: [item], width: item.width }])
  }

  search(0, recommendation ? [{ items: [recommendation], width: recommendation.width }] : [])
  const result = best ?? [{ items: normalized, width: normalized.reduce((total, item) => total + item.width, 0) }]
  return result.map(row => row.items.slice().sort((a, b) => (a.order ?? 0) - (b.order ?? 0)).map(item => item.id))
}
