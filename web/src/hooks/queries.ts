import { useQuery } from '@tanstack/react-query'
import {
  assignProductCategory,
  getImport,
  getProduct,
  getProductMetrics,
  listCategories,
  listImports,
  listMappings,
  listTrends,
  type TrendsParams,
} from '../api/client'

export function useTrends(params: TrendsParams) {
  return useQuery({
    queryKey: ['trends', params],
    queryFn: () => listTrends(params),
    placeholderData: (previous) => previous,
  })
}

export function useProduct(id: string | undefined) {
  return useQuery({
    queryKey: ['product', id],
    queryFn: () => getProduct(id as string),
    enabled: Boolean(id),
  })
}

export function useProductMetrics(id: string | undefined, window: string) {
  return useQuery({
    queryKey: ['product-metrics', id, window],
    queryFn: () => getProductMetrics(id as string, window),
    enabled: Boolean(id),
  })
}

export function useCategories() {
  return useQuery({
    queryKey: ['categories'],
    queryFn: listCategories,
    staleTime: 5 * 60_000,
  })
}

export function useMappings() {
  return useQuery({
    queryKey: ['mappings'],
    queryFn: listMappings,
    staleTime: 5 * 60_000,
  })
}

export function useImports(limit = 20) {
  return useQuery({
    queryKey: ['imports', limit],
    queryFn: () => listImports(limit),
  })
}

export function useImportDetail(id: string | undefined) {
  return useQuery({
    queryKey: ['import-detail', id],
    queryFn: () => getImport(id as string),
    enabled: Boolean(id),
  })
}

export function useAssignCategory() {
  // mutation hook helpers are kept simple; callers can use mutate directly.
  return async (productID: string, canonicalCategoryID: string, reviewer: string) => {
    return assignProductCategory(productID, canonicalCategoryID, reviewer)
  }
}