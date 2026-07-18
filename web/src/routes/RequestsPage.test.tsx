import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RequestsPage from './RequestsPage'
import { TestProviders } from '../test/TestProviders'
import { listMaterialRequests, getProduct } from '../api/client'
import type { ListResponse, MaterialRequest } from '../api/types'

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

const mockedList = vi.mocked(listMaterialRequests)
const mockedGetProduct = vi.mocked(getProduct)

const sampleRequest: MaterialRequest = {
  id: 'req-100',
  product_id: 'prod-1',
  client_id: '',
  usage: 'social',
  style: 'playful',
  focus: 'battery life',
  notes: '',
  status: 'generated',
  created_at: '2026-07-01T10:00:00Z',
  updated_at: '2026-07-01T10:05:00Z',
}

function listResponse(items: MaterialRequest[]): ListResponse<MaterialRequest> {
  return {
    items,
    pagination: {
      page: 1,
      page_size: 20,
      total_items: items.length,
      total_pages: 1,
      has_next: false,
      has_previous: false,
    },
  }
}

function renderPage(ui: ReactNode) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <TestProviders>
      <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
    </TestProviders>,
  )
}

describe('RequestsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedList.mockResolvedValue(listResponse([sampleRequest]))
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

  it('renders the request list with product title, status tag and links', async () => {
    renderPage(<RequestsPage />)

    const idLink = await screen.findByRole('link', { name: 'req-100' })
    expect(idLink).toHaveAttribute('href', '/requests/req-100')

    const productLink = await screen.findByRole('link', { name: 'Test Lamp' })
    expect(productLink).toHaveAttribute('href', '/products/prod-1')

    expect(screen.getByText('Generated')).toBeInTheDocument()
  })

  it('renders the empty state when there are no requests', async () => {
    mockedList.mockResolvedValue(listResponse([]))
    renderPage(<RequestsPage />)
    expect(await screen.findByText('No requests yet')).toBeInTheDocument()
  })
})
