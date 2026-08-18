import { describe, expect, it } from 'vitest'
import { acceptPeriod } from './periodSeq'

describe('acceptPeriod', () => {
  it('drops stale replies after a newer click', () => {
    expect(acceptPeriod(3, 2)).toBe(false)
    expect(acceptPeriod(3, 3)).toBe(true)
  })
})
