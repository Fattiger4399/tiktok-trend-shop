import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  approveRequest,
  assignProductCategory,
  createMaterialRequest,
  deliverRequest,
  generateRequestCopy,
  getImport,
  getMaterialRequest,
  getProduct,
  getProductMetrics,
  getRequestReview,
  listCategories,
  listDeliveries,
  listImports,
  listMappings,
  listMarketplaces,
  listMaterialRequests,
  listRequestVariants,
  listTrends,
  rejectRequest,
  type CreateMaterialRequestParams,
  type ListDeliveriesParams,
  type ListMaterialRequestsParams,
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

export function useMarketplaces() {
  return useQuery({
    queryKey: ['marketplaces'],
    queryFn: listMarketplaces,
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

export function useMaterialRequests(params: ListMaterialRequestsParams) {
  return useQuery({
    queryKey: ['requests', params],
    queryFn: () => listMaterialRequests(params),
    placeholderData: (previous) => previous,
  })
}

export function useMaterialRequest(id: string | undefined) {
  return useQuery({
    queryKey: ['request', id],
    queryFn: () => getMaterialRequest(id as string),
    enabled: Boolean(id),
  })
}

export function useRequestReview(id: string | undefined) {
  return useQuery({
    queryKey: ['request-review', id],
    queryFn: () => getRequestReview(id as string),
    enabled: Boolean(id),
  })
}

export function useRequestVariants(id: string | undefined) {
  return useQuery({
    queryKey: ['request-variants', id],
    queryFn: () => listRequestVariants(id as string),
    enabled: Boolean(id),
  })
}

export function useDeliveries(params: ListDeliveriesParams) {
  return useQuery({
    queryKey: ['deliveries', params],
    queryFn: () => listDeliveries(params),
    placeholderData: (previous) => previous,
  })
}

function useInvalidateRequest(id: string) {
  const queryClient = useQueryClient()
  return () => {
    queryClient.invalidateQueries({ queryKey: ['request-review', id] })
    queryClient.invalidateQueries({ queryKey: ['request-variants', id] })
    queryClient.invalidateQueries({ queryKey: ['request', id] })
    queryClient.invalidateQueries({ queryKey: ['requests'] })
    queryClient.invalidateQueries({ queryKey: ['deliveries'] })
  }
}

export function useCreateMaterialRequest() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (params: CreateMaterialRequestParams) => createMaterialRequest(params),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['requests'] })
    },
  })
}

export function useGenerateRequestCopy(id: string) {
  const invalidate = useInvalidateRequest(id)
  return useMutation({
    mutationFn: () => generateRequestCopy(id),
    onSuccess: invalidate,
  })
}

export function useApproveRequest(id: string) {
  const invalidate = useInvalidateRequest(id)
  return useMutation({
    mutationFn: (params: { variant_id: string; actor: string; note?: string }) =>
      approveRequest(id, params),
    onSuccess: invalidate,
  })
}

export function useRejectRequest(id: string) {
  const invalidate = useInvalidateRequest(id)
  return useMutation({
    mutationFn: (params: { reason: string; actor: string }) => rejectRequest(id, params),
    onSuccess: invalidate,
  })
}

export function useDeliverRequest(id: string) {
  const invalidate = useInvalidateRequest(id)
  return useMutation({
    mutationFn: (params: { variant_id: string; actor: string }) => deliverRequest(id, params),
    onSuccess: invalidate,
  })
}