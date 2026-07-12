import { describe, expect, it } from 'vitest'
import { escapeText, isSpreadsheetFormula } from './escape'

describe('escape utilities', () => {
  it('escapes HTML angle brackets', () => {
    expect(escapeText('<script>alert(1)</script>')).toBe('&lt;script&gt;alert(1)&lt;/script&gt;')
  })

  it('escapes ampersands', () => {
    expect(escapeText('A & B')).toBe('A &amp; B')
  })

  it('returns empty string for null/undefined', () => {
    expect(escapeText(null)).toBe('')
    expect(escapeText(undefined)).toBe('')
  })

  it('detects spreadsheet formulas', () => {
    expect(isSpreadsheetFormula('=SUM(A1:A2)')).toBe(true)
    expect(isSpreadsheetFormula('+1+1')).toBe(true)
    expect(isSpreadsheetFormula('@cmd')).toBe(true)
    expect(isSpreadsheetFormula('safe text')).toBe(false)
  })
})