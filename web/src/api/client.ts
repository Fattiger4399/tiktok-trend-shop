import type {
  CanonicalCategory,
  CategoryAssignment,
  CopyVariant,
  Delivery,
  ImportJob,
  ImportRowOutcome,
  ListResponse,
  MaterialRequest,
  PrefillSuggestion,
  ProductResult,
  RequestReview,
  ReviewEvent,
  ScoreSummary,
  SnapshotRecord,
  SourceMapping,
} from './types'

const BASE = '/api/v1'
const TOKEN_KEY = 'tts_token'

export function getToken(): string | null {
  return window.localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  window.localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  window.localStorage.removeItem(TOKEN_KEY)
}

export class APIException extends Error {
  status: number
  code: string
  field?: string
  constructor(status: number, code: string, message: string, field?: string) {
    super(message)
    this.status = status
    this.code = code
    this.field = field
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getToken()
  const res = await fetch(`${BASE}${path}`, {
    headers: {
      Accept: 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init?.body && !(init.body instanceof FormData) ? { 'Content-Type': 'application/json' } : {}),
      ...(init?.headers ?? {}),
    },
    ...init,
  })
  if (res.status === 401) {
    // The token is gone or expired: drop it and force a re-login.
    clearToken()
    if (!window.location.pathname.startsWith('/login')) {
      window.location.assign('/login')
    }
  }
  if (!res.ok) {
    let body: { error?: { code: string; message: string; field?: string } } = {}
    try {
      body = await res.json()
    } catch {
      // ignore body parse failures
    }
    throw new APIException(
      res.status,
      body.error?.code ?? 'http_error',
      body.error?.message ?? res.statusText,
      body.error?.field,
    )
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export interface AuthUser {
  id: string
  username: string
  role: 'operator' | 'client'
  display_name: string
  active?: boolean
  created_at?: string
}

export interface LoginResponse {
  token: string
  user: AuthUser
}

export function login(username: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export function logout(): Promise<void> {
  return request<void>('/auth/logout', { method: 'POST' })
}

export function fetchMe(): Promise<AuthUser> {
  return request<AuthUser>('/auth/me')
}

export interface CreateUserParams {
  username: string
  password: string
  role: 'operator' | 'client'
  display_name?: string
}

export function createUser(params: CreateUserParams): Promise<AuthUser> {
  return request<AuthUser>('/users', {
    method: 'POST',
    body: JSON.stringify(params),
  })
}

export function listUsers(): Promise<ListResponse<AuthUser>> {
  return request<ListResponse<AuthUser>>('/users')
}

export interface TrendsParams {
  page?: number
  page_size?: number
  sort?: 'score' | 'price' | 'updated' | 'created' | 'asin'
  direction?: 'asc' | 'desc'
  q?: string
  category?: string
  marketplace?: string
  window?: '24h' | '7d' | '30d' | '90d' | 'all'
}

export function listTrends(params: TrendsParams = {}): Promise<ListResponse<ProductResult>> {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') search.append(key, String(value))
  })
  const query = search.toString()
  return request<ListResponse<ProductResult>>(`/trends${query ? `?${query}` : ''}`)
}

export interface ProductDetail {
  product: {
    id: string
    title: string
    region: string
    marketplace?: string | null
    asin?: string | null
    category?: string | null
    canonical_url?: string | null
  }
  detail: {
    product_url?: string | null
    image_url?: string | null
    price?: number | null
    currency?: string | null
    selling_points?: string[]
    review_summary?: string | null
    shop_name?: string | null
    brand?: string | null
    platform?: string | null
    review_highlights?: string[]
    completeness: string
    missing_fields: string[]
    captured_at: string
  } | null
  latest_snapshot: SnapshotRecord | null
  score: ScoreSummary | null
  assignment: CategoryAssignment | null
}

export function getProduct(id: string): Promise<ProductDetail> {
  return request<ProductDetail>(`/products/${encodeURIComponent(id)}`)
}

export interface ProductMetricsResponse {
  product_id: string
  window: string
  items: SnapshotRecord[]
}

export function getProductMetrics(id: string, window: string = '30d'): Promise<ProductMetricsResponse> {
  return request<ProductMetricsResponse>(`/products/${encodeURIComponent(id)}/metrics?window=${window}`)
}

export function listCategories(): Promise<ListResponse<CanonicalCategory>> {
  return request<ListResponse<CanonicalCategory>>('/categories')
}

export function listMarketplaces(): Promise<ListResponse<string>> {
  return request<ListResponse<string>>('/marketplaces')
}

export function listMappings(): Promise<ListResponse<SourceMapping>> {
  return request<ListResponse<SourceMapping>>('/category-mappings')
}

export function assignProductCategory(
  productID: string,
  canonicalCategoryID: string,
  reviewer: string,
): Promise<{ product_id: string; assignment: CategoryAssignment }> {
  return request(`/products/${encodeURIComponent(productID)}/category`, {
    method: 'PATCH',
    body: JSON.stringify({ canonical_category_id: canonicalCategoryID, reviewer }),
  })
}

export function listImports(limit: number = 20): Promise<ListResponse<ImportJob>> {
  return request<ListResponse<ImportJob>>(`/imports?limit=${limit}`)
}

export function getImport(
  jobID: string,
): Promise<{ job: ImportJob; results: ImportRowOutcome[] }> {
  return request(`/imports/${encodeURIComponent(jobID)}`)
}

export interface CreateImportParams {
  file: File
  region?: string
  idempotencyKey?: string
}

export interface CreateImportResult {
  job_id: string
  imported_rows: number
  updated_rows: number
  duplicate_rows: number
  rejected_rows: number
  total_rows: number
  status: string
}

export async function createImport(params: CreateImportParams): Promise<CreateImportResult> {
  const form = new FormData()
  form.append('file', params.file)
  if (params.region) form.append('region', params.region)
  if (params.idempotencyKey) form.append('idempotency_key', params.idempotencyKey)
  return request<CreateImportResult>('/imports/csv', {
    method: 'POST',
    body: form,
  })
}

export interface CreateMaterialRequestParams {
  product_id: string
  usage: string
  style: string
  focus: string
  notes: string
  client_id?: string
}

export function createMaterialRequest(params: CreateMaterialRequestParams): Promise<MaterialRequest> {
  return request<MaterialRequest>('/requests', {
    method: 'POST',
    body: JSON.stringify(params),
  })
}

export interface ListMaterialRequestsParams {
  status?: string
  product_id?: string
  page?: number
  page_size?: number
}

export function listMaterialRequests(
  params: ListMaterialRequestsParams = {},
): Promise<ListResponse<MaterialRequest>> {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') search.append(key, String(value))
  })
  const query = search.toString()
  return request<ListResponse<MaterialRequest>>(`/requests${query ? `?${query}` : ''}`)
}

export function getMaterialRequest(id: string): Promise<MaterialRequest> {
  return request<MaterialRequest>(`/requests/${encodeURIComponent(id)}`)
}

export function prefillRequestBrief(
  productID: string,
): Promise<{ product_id: string; suggestion: PrefillSuggestion }> {
  return request(`/products/${encodeURIComponent(productID)}/prefill`, {
    method: 'POST',
  })
}

export function generateRequestCopy(
  id: string,
): Promise<{ request_id: string; items: CopyVariant[] }> {
  return request(`/requests/${encodeURIComponent(id)}/generate`, {
    method: 'POST',
  })
}

export function listRequestVariants(
  id: string,
): Promise<{ request_id: string; items: CopyVariant[] }> {
  return request(`/requests/${encodeURIComponent(id)}/variants`)
}

export function approveRequest(
  id: string,
  params: { variant_id: string; actor: string; note?: string },
): Promise<{ request: MaterialRequest; event: ReviewEvent }> {
  return request(`/requests/${encodeURIComponent(id)}/approve`, {
    method: 'POST',
    body: JSON.stringify(params),
  })
}

export function rejectRequest(
  id: string,
  params: { reason: string; actor: string },
): Promise<{ request: MaterialRequest; event: ReviewEvent }> {
  return request(`/requests/${encodeURIComponent(id)}/reject`, {
    method: 'POST',
    body: JSON.stringify(params),
  })
}

export function deliverRequest(
  id: string,
  params: { variant_id: string; actor: string },
): Promise<{ request: MaterialRequest; delivery: Delivery }> {
  return request(`/requests/${encodeURIComponent(id)}/deliver`, {
    method: 'POST',
    body: JSON.stringify(params),
  })
}

export function getRequestReview(id: string): Promise<RequestReview> {
  return request<RequestReview>(`/requests/${encodeURIComponent(id)}/review`)
}

export interface ListDeliveriesParams {
  page?: number
  page_size?: number
}

export function listDeliveries(params: ListDeliveriesParams = {}): Promise<ListResponse<Delivery>> {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') search.append(key, String(value))
  })
  const query = search.toString()
  return request<ListResponse<Delivery>>(`/deliveries${query ? `?${query}` : ''}`)
}

export function getDelivery(id: string): Promise<Delivery> {
  return request<Delivery>(`/deliveries/${encodeURIComponent(id)}`)
}