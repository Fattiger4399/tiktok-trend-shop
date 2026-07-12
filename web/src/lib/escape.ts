export function escapeText(value: unknown): string {
  if (value === null || value === undefined) return ''
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export function isSpreadsheetFormula(value: unknown): boolean {
  if (typeof value !== 'string') return false
  if (value.length === 0) return false
  const first = value[0]
  return first === '=' || first === '+' || first === '-' || first === '@'
}