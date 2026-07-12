import { useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { TrendsParams } from '../api/client'

export interface UseTrendsURL {
  params: TrendsParams
  setParam: <K extends keyof TrendsParams>(key: K, value: TrendsParams[K]) => void
  clear: () => void
}

export function useTrendsURL(): UseTrendsURL {
  const [search, setSearch] = useSearchParams()

  const params: TrendsParams = {
    page: numberParam(search, 'page', 1),
    page_size: numberParam(search, 'page_size', 20),
    sort: (search.get('sort') as TrendsParams['sort']) ?? 'score',
    direction: (search.get('direction') as TrendsParams['direction']) ?? 'desc',
    q: search.get('q') ?? '',
    category: search.get('category') ?? '',
    marketplace: search.get('marketplace') ?? '',
    window: (search.get('window') as TrendsParams['window']) ?? '30d',
  }

  const setParam = <K extends keyof TrendsParams>(key: K, value: TrendsParams[K]) => {
    const next = new URLSearchParams(search)
    if (value === undefined || value === '' || value === null) {
      next.delete(key)
    } else {
      next.set(key, String(value))
    }
    if (key !== 'page') next.delete('page')
    setSearch(next, { replace: true })
  }

  const clear = () => {
    setSearch(new URLSearchParams(), { replace: true })
  }

  useEffect(() => {
    // Reset page to 1 when other params change is handled inside setParam.
  }, [search])

  return { params, setParam, clear }
}

function numberParam(search: URLSearchParams, key: string, fallback: number): number {
  const raw = search.get(key)
  if (!raw) return fallback
  const n = Number(raw)
  return Number.isFinite(n) && n > 0 ? n : fallback
}