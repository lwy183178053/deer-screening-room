import { describe, expect, it } from 'vitest'
import { packStudioRows } from './studioLayout'

describe('studio layout packing', () => {
  it('keeps recommendation at the start of the first row', () => {
    expect(packStudioRows([
      { id: 0, width: 30 },
      { id: 1, width: 60 },
      { id: 2, width: 40 },
      { id: 3, width: 20 },
    ], 100, 10)).toHaveLength(2)
    expect(packStudioRows([
      { id: 0, width: 30 },
      { id: 1, width: 60 },
      { id: 2, width: 40 },
      { id: 3, width: 20 },
    ], 100, 10).flat()).toEqual(expect.arrayContaining([0, 1, 2, 3]))
  })

  it('minimizes the number of rows before balancing empty space', () => {
    expect(packStudioRows([
      { id: 0, width: 30 },
      { id: 1, width: 55 },
      { id: 2, width: 45 },
      { id: 3, width: 35 },
    ], 100, 10)).toHaveLength(2)
  })

  it('returns stable rows for the same measurements', () => {
    const items = [{ id: 0, width: 30 }, { id: 1, width: 70 }, { id: 2, width: 40 }, { id: 3, width: 20 }]
    expect(packStudioRows(items, 120, 8)).toEqual(packStudioRows(items, 120, 8))
  })

  it('keeps every item when one label is wider than the container', () => {
    expect(packStudioRows([{ id: 0, width: 30 }, { id: 1, width: 180 }, { id: 2, width: 30 }], 100, 8).flat()).toEqual(expect.arrayContaining([0, 1, 2]))
  })
})
