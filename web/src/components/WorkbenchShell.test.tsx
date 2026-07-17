import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import WorkbenchShell from '../components/WorkbenchShell'
import { TestProviders } from '../test/TestProviders'

describe('WorkbenchShell', () => {
  it('renders the brand and navigation', () => {
    render(
      <TestProviders>
        <WorkbenchShell />
      </TestProviders>,
    )
    expect(screen.getByText('Trend Workbench')).toBeInTheDocument()
    expect(screen.getAllByText('Trending').length).toBeGreaterThan(0)
    expect(screen.getByText('Categories')).toBeInTheDocument()
    expect(screen.getByText('Imports')).toBeInTheDocument()
  })

  it('shows English copy by default', () => {
    render(
      <TestProviders>
        <WorkbenchShell />
      </TestProviders>,
    )
    expect(screen.getByText('Local workbench')).toBeInTheDocument()
  })
})