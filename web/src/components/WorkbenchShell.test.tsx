import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import WorkbenchShell from '../components/WorkbenchShell'
import { TestProviders } from '../test/TestProviders'
import { fetchMe } from '../api/client'

vi.mock('../api/client', () => ({
  getToken: vi.fn(() => 'test-token'),
  setToken: vi.fn(),
  clearToken: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(() => Promise.resolve()),
  fetchMe: vi.fn(),
  createUser: vi.fn(),
  listUsers: vi.fn(),
}))

const mockedFetchMe = vi.mocked(fetchMe)

const operatorUser = {
  id: 'usr_1',
  username: 'admin',
  role: 'operator' as const,
  display_name: 'Admin',
}
const clientUser = {
  id: 'usr_2',
  username: 'client-a',
  role: 'client' as const,
  display_name: 'Client A',
}

describe('WorkbenchShell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedFetchMe.mockResolvedValue(operatorUser)
  })

  it('renders the brand and navigation', async () => {
    render(
      <TestProviders>
        <WorkbenchShell />
      </TestProviders>,
    )
    expect(screen.getByText('Trend Workbench')).toBeInTheDocument()
    expect(screen.getAllByText('Trending').length).toBeGreaterThan(0)
    expect(await screen.findByText('Categories')).toBeInTheDocument()
    expect(await screen.findByText('Imports')).toBeInTheDocument()
  })

  it('shows English copy by default', async () => {
    render(
      <TestProviders>
        <WorkbenchShell />
      </TestProviders>,
    )
    expect(screen.getByText('Local workbench')).toBeInTheDocument()
    // Flush the session restore so no state update lands after the test.
    expect(await screen.findByText(/Admin · Operator/)).toBeInTheDocument()
  })

  it('shows the current operator with a logout button', async () => {
    render(
      <TestProviders>
        <WorkbenchShell />
      </TestProviders>,
    )
    expect(await screen.findByText(/Admin · Operator/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Log out/ })).toBeInTheDocument()
  })

  it('hides operations menus for client users', async () => {
    mockedFetchMe.mockResolvedValue(clientUser)
    render(
      <TestProviders>
        <WorkbenchShell />
      </TestProviders>,
    )
    expect(await screen.findByText(/Client A · Client/)).toBeInTheDocument()
    expect(screen.getByText('Trending')).toBeInTheDocument()
    expect(screen.getByText('Requests')).toBeInTheDocument()
    expect(screen.queryByText('Imports')).not.toBeInTheDocument()
    expect(screen.queryByText('Categories')).not.toBeInTheDocument()
    expect(screen.queryByText('Deliveries')).not.toBeInTheDocument()
  })
})
