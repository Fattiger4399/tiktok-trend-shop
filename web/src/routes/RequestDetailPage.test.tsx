import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RequestDetailPage from './RequestDetailPage'
import { TestProviders } from '../test/TestProviders'
import { getProduct, getRequestReview, listRequestVariants } from '../api/client'
import type { CopyVariant, MaterialRequest, RequestReview } from '../api/types'

vi.mock('../api/client', () => ({
  getToken: vi.fn(() => null),
  setToken: vi.fn(),
  clearToken: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(() => Promise.resolve()),
  fetchMe: vi.fn(),
  createUser: vi.fn(),
  listUsers: vi.fn(),
  assignProductCategory: vi.fn(),
  getImport: vi.fn(),
  getProduct: vi.fn(),
  getProductMetrics: vi.fn(),
  listCategories: vi.fn(),
  listImports: vi.fn(),
  listMappings: vi.fn(),
  listMarketplaces: vi.fn(),
  listTrends: vi.fn(),
  createImport: vi.fn(),
  createMaterialRequest: vi.fn(),
  listMaterialRequests: vi.fn(),
  getMaterialRequest: vi.fn(),
  prefillRequestBrief: vi.fn(),
  generateRequestCopy: vi.fn(),
  listRequestVariants: vi.fn(),
  approveRequest: vi.fn(),
  rejectRequest: vi.fn(),
  deliverRequest: vi.fn(),
  getRequestReview: vi.fn(),
  listDeliveries: vi.fn(),
  getDelivery: vi.fn(),
}))

const mockedReview = vi.mocked(getRequestReview)
const mockedVariants = vi.mocked(listRequestVariants)
const mockedGetProduct = vi.mocked(getProduct)

function makeRequest(status: MaterialRequest['status']): MaterialRequest {
  return {
    id: 'req-1',
    product_id: 'prod-1',
    client_id: '',
    usage: 'social',
    style: 'playful',
    focus: 'battery life',
    notes: 'keep it short',
    status,
    created_at: '2026-07-01T10:00:00Z',
    updated_at: '2026-07-01T10:05:00Z',
  }
}

const sampleVariant: CopyVariant = {
  id: 'var-1',
  request_id: 'req-1',
  variant_no: 1,
  hook: 'Catchy hook',
  body: 'Body copy',
  caption: 'Caption copy',
  hashtags: ['#lamp'],
  provider: 'mock',
  model: 'mock-v1',
  prompt_version: 'v1',
  created_at: '2026-07-01T10:01:00Z',
}

function renderPage(ui: ReactNode, entry: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <TestProviders initialEntries={[entry]}>
      <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
    </TestProviders>,
  )
}

describe('RequestDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedGetProduct.mockResolvedValue({
      product: {
        id: 'prod-1',
        title: 'Test Lamp',
        region: 'CN',
        marketplace: 'Amazon',
        asin: 'B001',
      },
      detail: null,
      latest_snapshot: null,
      score: null,
      assignment: null,
    })
  })

  it('renders approve and reject actions when status is generated', async () => {
    const review: RequestReview = { request: makeRequest('generated'), events: [], delivery: null }
    mockedReview.mockResolvedValue(review)
    mockedVariants.mockResolvedValue({ request_id: 'req-1', items: [sampleVariant] })

    renderPage(
      <Routes>
        <Route path="/requests/:id" element={<RequestDetailPage />} />
      </Routes>,
      '/requests/req-1',
    )

    expect(await screen.findByRole('button', { name: 'Approve this variant' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument()
    expect(screen.getByText('Catchy hook')).toBeInTheDocument()
    expect(screen.getByText('#lamp')).toBeInTheDocument()
  })

  it('shows the latest reject reason and a generate button when rejected', async () => {
    const review: RequestReview = {
      request: makeRequest('rejected'),
      events: [
        {
          id: 'evt-1',
          request_id: 'req-1',
          action: 'rejected',
          actor: 'operator',
          note: 'copy is weak',
          variant_id: null,
          created_at: '2026-07-01T11:00:00Z',
        },
      ],
      delivery: null,
    }
    mockedReview.mockResolvedValue(review)
    mockedVariants.mockResolvedValue({ request_id: 'req-1', items: [] })

    renderPage(
      <Routes>
        <Route path="/requests/:id" element={<RequestDetailPage />} />
      </Routes>,
      '/requests/req-1',
    )

    expect(await screen.findByText('Latest rejection reason')).toBeInTheDocument()
    expect(screen.getByText('copy is weak')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Generate copy' })).toBeInTheDocument()
  })

  it('shows the deliver button when approved', async () => {
    const review: RequestReview = {
      request: makeRequest('approved'),
      events: [
        {
          id: 'evt-2',
          request_id: 'req-1',
          action: 'approved',
          actor: 'operator',
          note: 'looks good',
          variant_id: 'var-1',
          created_at: '2026-07-01T12:00:00Z',
        },
      ],
      delivery: null,
    }
    mockedReview.mockResolvedValue(review)
    mockedVariants.mockResolvedValue({ request_id: 'req-1', items: [sampleVariant] })

    renderPage(
      <Routes>
        <Route path="/requests/:id" element={<RequestDetailPage />} />
      </Routes>,
      '/requests/req-1',
    )

    expect(await screen.findByRole('button', { name: 'Finalize & deliver' })).toBeInTheDocument()
  })
})
