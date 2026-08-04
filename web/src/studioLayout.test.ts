import { describe, expect, it } from 'vitest'
import { packStudioItems } from './studioLayout'

const studios = [
  { id: 1, width: 70 },
  { id: 2, width: 50 },
  { id: 3, width: 20 },
]

describe('studio layout packing', () => {
  it('keeps recommendation first and backfills its row with the best fitting studio', () => {
    expect(packStudioItems(studios, 100, 30, 10)).toEqual([2, 1, 3])
  })

  it('anchors each new row with the earliest remaining studio', () => {
    expect(packStudioItems([
      { id: 1, width: 55 },
      { id: 2, width: 40 },
      { id: 3, width: 35 },
    ], 100, 0, 10)).toEqual([1, 3, 2])
  })

  it('returns the same order for the same measurements', () => {
    const first = packStudioItems(studios, 100, 30, 10)
    const second = packStudioItems(studios, 100, 30, 10)
    expect(second).toEqual(first)
  })

  it('reflows when the available width changes', () => {
    expect(packStudioItems(studios, 100, 30, 10)).toEqual([2, 1, 3])
    expect(packStudioItems(studios, 120, 30, 10)).toEqual([1, 2, 3])
  })

  it('places an oversized studio on its own row without losing following studios', () => {
    expect(packStudioItems([
      { id: 1, width: 160 },
      { id: 2, width: 30 },
    ], 100, 0, 8)).toEqual([1, 2])
  })
})
