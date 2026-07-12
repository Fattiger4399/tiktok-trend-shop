import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import WorkbenchShell from '../components/WorkbenchShell'

describe('WorkbenchShell', () => {
  it('renders the brand and navigation', () => {
    render(
      <MemoryRouter initialEntries={['/trends']}>
        <WorkbenchShell />
      </MemoryRouter>,
    )
    expect(screen.getByText('Trend Workbench')).toBeInTheDocument()
    expect(screen.getByText('Trending')).toBeInTheDocument()
    expect(screen.getByText('Categories')).toBeInTheDocument()
    expect(screen.getByText('Imports')).toBeInTheDocument()
  })
})