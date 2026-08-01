import { describe, expect, it } from 'vitest'
import { formatBytes, formatDuration } from './format'

describe('format helpers', () => {
  it('formats playback duration', () => {
    expect(formatDuration(65_000)).toBe('1:05')
    expect(formatDuration(3_665_000)).toBe('1:01:05')
  })

  it('formats byte sizes', () => {
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(568_611_764)).toBe('542 MB')
  })
})

