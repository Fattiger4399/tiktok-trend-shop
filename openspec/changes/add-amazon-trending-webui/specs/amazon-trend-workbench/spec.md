## ADDED Requirements

### Requirement: Trending product dashboard
The system SHALL provide a browser-based dashboard that presents trending products in a scan-friendly workbench rather than a marketing landing page.

#### Scenario: Dashboard loads available products
- **WHEN** a user opens the trending products route
- **THEN** the system displays product identity, category, price when available, hotspot score, trend indicators, data source, and last update time

#### Scenario: Product has incomplete data
- **WHEN** a product is missing an image, price, ASIN, score component, or historical metric
- **THEN** the dashboard marks the field as unavailable and does not represent the missing value as zero

### Requirement: Dashboard filtering and sorting
The system SHALL allow users to filter and sort the trend list without losing the active selection when navigating through paginated results.

#### Scenario: User filters the dashboard
- **WHEN** a user selects a marketplace, canonical category, time window, or search term
- **THEN** the system requests and displays only matching products and reflects the active filters in the route state

#### Scenario: User sorts by a supported metric
- **WHEN** a user selects hotspot score, rank movement, review growth, price, or update time sorting
- **THEN** the system displays products in the server-provided order and indicates the active direction

#### Scenario: User clears filters
- **WHEN** a user activates the clear-filters command
- **THEN** the system restores the default marketplace, time window, sort order, and first page

### Requirement: Product detail experience
The system SHALL provide a dedicated product detail route containing normalized product information, source provenance, classification, hotspot explanation, and metric history.

#### Scenario: User opens a product
- **WHEN** a user selects a product from the dashboard
- **THEN** the system navigates to a shareable product URL and displays the latest normalized product details

#### Scenario: Historical metrics are available
- **WHEN** the selected product has multiple timestamped metric observations
- **THEN** the detail route displays labeled history charts for available price, rank, review, or demand metrics

#### Scenario: Historical metrics are insufficient
- **WHEN** the selected product has fewer observations than required for a trend chart
- **THEN** the system displays a concise insufficient-history state and preserves the latest known metric

### Requirement: Explainable trend presentation
The system SHALL distinguish observed metrics, estimated metrics, derived hotspot components, and missing evidence.

#### Scenario: Estimated sales are displayed
- **WHEN** a product contains a provider-supplied sales estimate
- **THEN** the UI labels the value as estimated and displays its provider and observation time

#### Scenario: Hotspot score confidence is reduced
- **WHEN** the hotspot score lacks sufficient historical or category comparison data
- **THEN** the UI displays the reduced confidence and identifies unavailable score components

### Requirement: Responsive and accessible workbench states
The system SHALL remain usable on desktop and mobile viewports and SHALL expose meaningful loading, empty, stale, and error states.

#### Scenario: Dashboard is viewed on a narrow viewport
- **WHEN** the viewport cannot support the desktop table columns
- **THEN** the system uses stable product rows that preserve title, image, primary metric, hotspot score, and navigation without horizontal content overlap

#### Scenario: Trend request is loading
- **WHEN** a dashboard or detail request is pending
- **THEN** the system displays a layout-stable loading state and keeps available navigation controls operable

#### Scenario: Trend request fails
- **WHEN** the API returns an error or cannot be reached
- **THEN** the system displays the failure, provides a retry command, and does not present stale data as freshly updated

