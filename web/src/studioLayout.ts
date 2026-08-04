export type StudioLayoutItem = {
  id: number
  width: number
}

export function packStudioItems(
  items: readonly StudioLayoutItem[],
  containerWidth: number,
  leadingWidth: number,
  gap = 8,
): number[] {
  if (containerWidth <= 0) return items.map(item => item.id)

  const remaining = items.map(item => ({ ...item, width: Math.min(Math.max(item.width, 0), containerWidth) }))
  const result: number[] = []
  const rowGap = Math.max(gap, 0)
  let used = Math.min(Math.max(leadingWidth, 0), containerWidth)

  const fits = (index: number) => used + (used > 0 ? rowGap : 0) + remaining[index].width <= containerWidth + 0.5
  const bestFit = () => {
    let best = -1
    for (let index = 0; index < remaining.length; index++) {
      if (!fits(index)) continue
      if (best === -1 || remaining[index].width > remaining[best].width) best = index
    }
    return best
  }
  const place = (index: number) => {
    const [item] = remaining.splice(index, 1)
    used += (used > 0 ? rowGap : 0) + item.width
    result.push(item.id)
  }

  while (remaining.length > 0) {
    let next = used === 0 ? 0 : (fits(0) ? 0 : bestFit())
    if (next === -1) {
      used = 0
      continue
    }
    place(next)
    while ((next = bestFit()) !== -1) place(next)
    used = 0
  }

  return result
}
