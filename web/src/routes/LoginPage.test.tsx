import type { ReactNode } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import LoginPage from './LoginPage'
import { RequireAuth } from '../auth/RequireAuth'
import { TestProviders } from '../test/TestProviders'
import { login } from '../api/client'

vi.mock('../api/client', () => ({
  getToken: vi.fn(() => null),
  setToken: vi.fn(),
  clearToken: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(() => Promise.resolve()),
  fetchMe: vi.fn(),
  createUser: vi.fn(),
  listUsers: vi.fn(),
}))

const mockedLogin = vi.mocked(login)

function renderAt(ui: ReactNode, entry: string) {
  return render(<TestProviders initialEntries={[entry]}>{ui}</TestProviders>)
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the login form', async () => {
    renderAt(
      <Routes>
        <Route path="/login" element={<LoginPage />} />
      </Routes>,
      '/login',
    )
    expect(await screen.findByText('Sign in to the Trend Workbench')).toBeInTheDocument()
    expect(screen.getByLabelText('Username')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument()
  })

  it('redirects unauthenticated users to /login', async () => {
    renderAt(
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/trends"
          element={
            <RequireAuth>
              <div>Protected trends</div>
            </RequireAuth>
          }
        />
      </Routes>,
      '/trends',
    )
    expect(await screen.findByText('Sign in to the Trend Workbench')).toBeInTheDocument()
    expect(screen.queryByText('Protected trends')).not.toBeInTheDocument()
  })

  it('submits credentials and navigates away on success', async () => {
    mockedLogin.mockResolvedValue({
      token: 'tok-1',
      user: { id: 'usr_1', username: 'admin', role: 'operator', display_name: 'Admin' },
    })
    renderAt(
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/trends" element={<div>Trends page</div>} />
      </Routes>,
      '/login',
    )
    fireEvent.change(await screen.findByLabelText('Username'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'admin123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
    await waitFor(() => expect(mockedLogin).toHaveBeenCalledWith('admin', 'admin123'))
    expect(await screen.findByText('Trends page')).toBeInTheDocument()
  })

  it('shows the server error when login fails', async () => {
    mockedLogin.mockRejectedValue(new Error('invalid username or password'))
    renderAt(
      <Routes>
        <Route path="/login" element={<LoginPage />} />
      </Routes>,
      '/login',
    )
    fireEvent.change(await screen.findByLabelText('Username'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrong' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByText('invalid username or password')).toBeInTheDocument()
  })
})
