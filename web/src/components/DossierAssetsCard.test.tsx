import { fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DossierAssetsCard } from './DossierAssetsCard'
import { TestProviders } from '../test/TestProviders'
import { addDossierAsset, listDossierAssets } from '../api/client'
import type { DossierAsset } from '../api/types'

vi.mock('../api/client', () => ({
  getToken: vi.fn(() => null),
  setToken: vi.fn(),
  clearToken: vi.fn(),
  fetchMe: vi.fn(),
  listDossierAssets: vi.fn(),
  addDossierAsset: vi.fn(),
  deleteDossierAsset: vi.fn(),
}))

const mockedList = vi.mocked(listDossierAssets)
const mockedAdd = vi.mocked(addDossierAsset)

const sampleAssets: DossierAsset[] = [
  {
    id: 'da_1',
    product_id: 'prod-1',
    kind: 'image',
    url: 'https://example.com/lamp.jpg',
    content: '',
    source: 'amazon.com',
    note: 'hero shot',
    created_by: 'admin',
    created_at: '2026-07-01T10:00:00Z',
  },
  {
    id: 'da_2',
    product_id: 'prod-1',
    kind: 'text',
    url: '',
    content: 'Buyers love the soft light',
    source: '1688',
    note: '',
    created_by: 'admin',
    created_at: '2026-07-01T11:00:00Z',
  },
  {
    id: 'da_3',
    product_id: 'prod-1',
    kind: 'link',
    url: 'https://example.com/item/1',
    content: '',
    source: 'amazon.com',
    note: 'competitor listing',
    created_by: 'admin',
    created_at: '2026-07-01T12:00:00Z',
  },
]

function renderCard() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <TestProviders initialEntries={['/products/prod-1']}>
      <QueryClientProvider client={queryClient}>
        <DossierAssetsCard productID="prod-1" />
      </QueryClientProvider>
    </TestProviders>,
  )
}

describe('DossierAssetsCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders image, text and link assets with provenance', async () => {
    mockedList.mockResolvedValue({ product_id: 'prod-1', items: sampleAssets })

    renderCard()

    expect(await screen.findByText('Web Assets')).toBeInTheDocument()
    // image asset renders a thumbnail pointing at the url
    const img = await screen.findByRole('img', { name: 'hero shot' })
    expect(img).toHaveAttribute('src', 'https://example.com/lamp.jpg')
    // text asset renders the quote block
    expect(screen.getByText('Buyers love the soft light')).toBeInTheDocument()
    // link asset renders a clickable hyperlink
    const link = screen.getByRole('link', { name: 'https://example.com/item/1' })
    expect(link).toHaveAttribute('href', 'https://example.com/item/1')
    // provenance and delete actions are visible
    expect(screen.getAllByText('amazon.com').length).toBe(2)
    expect(screen.getByText('1688')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(3)
    expect(mockedList).toHaveBeenCalledWith('prod-1', undefined)
  })

  it('blocks submit when an image asset has no url', async () => {
    mockedList.mockResolvedValue({ product_id: 'prod-1', items: [] })

    renderCard()

    const submit = await screen.findByRole('button', { name: 'Add asset' })
    fireEvent.click(submit)

    expect(await screen.findByText('Image/link assets require a URL')).toBeInTheDocument()
    expect(mockedAdd).not.toHaveBeenCalled()
  })
})
