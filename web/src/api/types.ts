export interface Pagination {
  page: number
  page_size: number
  total_items: number
  total_pages: number
  has_next: boolean
  has_previous: boolean
}

export interface ListResponse<T> {
  items: T[]
  pagination: Pagination
  effective?: Record<string, unknown>
  warnings?: APIError[]
}

export interface APIError {
  code: string
  message: string
  field?: string
}

export interface Provenance {
  provider: string
  source_type: string
  source_id?: string | null
  source_url?: string | null
  captured_at: string
}

export interface Freshness {
  last_captured_at?: string | null
  stale: boolean
  threshold_hours: number
}

export interface ScoreSummary {
  id: string
  total_score: number
  confidence: string
  model_version: string
  time_window: string
  comparison_group?: string
  components: Record<string, number>
  missing_components: string[]
  missing_detail_fields?: string[]
  created_at: string
}

export interface CanonicalRef {
  id: string
  name: string
  slug: string
  confidence: number
  method: string
  manual: boolean
}

export interface ProductResult {
  id: string
  title: string
  region: string
  marketplace?: string | null
  asin?: string | null
  source_category?: string | null
  source_category_id?: string | null
  category?: string | null
  canonical_category?: CanonicalRef | null
  workflow_state: string
  canonical_url?: string | null
  metrics: Record<string, number>
  metric_kind: string
  estimated: boolean
  provenance: Provenance
  freshness: Freshness
  score?: ScoreSummary | null
  detail_snapshot_id?: string | null
  detail_completeness: string
  detail_missing_fields: string[]
  product_url?: string | null
  image_url?: string | null
  platform?: string | null
  shop_name?: string | null
  brand?: string | null
  price?: number | null
  currency?: string | null
  selling_points?: string[]
  specs?: Record<string, unknown>
  review_summary?: string | null
  review_highlights?: string[]
}

export interface CanonicalCategory {
  id: string
  parent_id?: string | null
  name: string
  slug: string
  display_order: number
  active: boolean
}

export interface SourceMapping {
  id: string
  provider: string
  source_category_id: string
  source_category_name?: string | null
  canonical_category_id: string
  confidence: number
  method: string
  active: boolean
}

export interface CategoryAssignment {
  id: string
  product_id: string
  canonical_category_id?: string | null
  method: string
  confidence: number
  source_category_id?: string | null
  source_category_name?: string | null
  manual: boolean
  needs_review: boolean
  assigned_at: string
  updated_at: string
}

export interface ImportJob {
  id: string
  source: string
  filename?: string | null
  status: string
  total_rows: number
  imported_rows: number
  updated_rows: number
  duplicate_rows: number
  rejected_rows: number
  started_at: string
  completed_at?: string | null
  idempotency_key?: string | null
}

export interface ImportRowOutcome {
  row_number: number
  status: string
  product_id?: string
  reason?: string
  field?: string
  raw?: Record<string, string>
}

export interface SnapshotRecord {
  id: string
  product_id: string
  provider: string
  source_type: string
  source_id?: string | null
  source_url?: string | null
  captured_at: string
  metric_kind: string
  metrics: Record<string, number>
}

export type MaterialRequestStatus =
  | 'submitted'
  | 'generating'
  | 'generated'
  | 'approved'
  | 'rejected'
  | 'delivered'

export interface MaterialRequest {
  id: string
  product_id: string
  client_id: string
  usage: string
  style: string
  focus: string
  notes: string
  status: MaterialRequestStatus
  created_at: string
  updated_at: string
}

export interface CopyVariant {
  id: string
  request_id: string
  variant_no: number
  hook: string
  body: string
  caption: string
  hashtags: string[]
  provider: string
  model: string
  prompt_version: string
  created_at: string
}

export interface ReviewEvent {
  id: string
  request_id: string
  action: 'approved' | 'rejected'
  actor: string
  note: string
  variant_id: string | null
  created_at: string
}

export interface DeliveryPackageProduct {
  id: string
  title: string
  asin?: string | null
  marketplace?: string | null
}

export interface DeliveryPackage {
  request: MaterialRequest
  variant: CopyVariant
  product: DeliveryPackageProduct
}

export interface Delivery {
  id: string
  request_id: string
  variant_id: string
  actor: string
  package: DeliveryPackage
  created_at: string
}

export interface RequestReview {
  request: MaterialRequest
  events: ReviewEvent[]
  delivery: Delivery | null
}

export interface PrefillSuggestion {
  usage: string
  style: string
  focus: string
  notes: string
}

export type DossierAssetKind = 'image' | 'text' | 'link'

export interface DossierAsset {
  id: string
  product_id: string
  kind: DossierAssetKind
  url: string
  content: string
  source: string
  note: string
  created_by: string
  created_at: string
}