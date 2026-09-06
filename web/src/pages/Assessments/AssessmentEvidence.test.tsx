import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, expect, test, vi } from 'vitest';
import { AssessmentEvidence } from './AssessmentEvidence';
import { assessmentsApi } from '../../api/assessments';
import type { EvidenceBundle } from '../../api/assessments';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function renderWithClient(id = '1') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <MemoryRouter initialEntries={[`/assessments/${id}/evidence`]}>
      <QueryClientProvider client={queryClient}>
        <Routes>
          <Route path="/assessments/:id/evidence" element={<AssessmentEvidence />} />
        </Routes>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

function mockGetEvidence(bundle: EvidenceBundle) {
  return vi.spyOn(assessmentsApi, 'getEvidence').mockResolvedValue({
    data: bundle,
  } as Awaited<ReturnType<typeof assessmentsApi.getEvidence>>);
}

const baseAssessment = {
  id: 1,
  name: 'Invoice Fraud Resilience',
  skill_tag: 'invoice-fraud',
  baseline_scenario_id: 10,
  followup_scenario_id: 11,
  benign_scenario_id: 12,
  observation_window_hours: 72,
  status: 'active',
  created_at: '2026-09-01T00:00:00Z',
};

test('renders a threat phase and a benign control phase with their metrics', async () => {
  mockGetEvidence({
    bundle_version: 1,
    generated_at: '2026-09-06T00:00:00Z',
    assessment: baseAssessment,
    observation_window_hours: 72,
    phases: [
      {
        phase: 'baseline',
        campaign_id: 100,
        recognition: {
          numerator: 2,
          denominator: 6,
          rate: 0.3333,
          ci_low: 0.138,
          ci_high: 0.609,
          suppressed: false,
        },
      },
      {
        phase: 'benign_control',
        campaign_id: 101,
        discrimination: {
          numerator: 4,
          denominator: 10,
          rate: 0.4,
          ci_low: 0.19,
          ci_high: 0.65,
          suppressed: false,
        },
      },
    ],
    limitations: ['This is an observational comparison, not a controlled experiment.'],
  });

  renderWithClient();

  expect(await screen.findByText('Invoice Fraud Resilience')).toBeInTheDocument();
  expect(screen.getByText('Baseline')).toBeInTheDocument();
  expect(screen.getByText('Benign Control')).toBeInTheDocument();
  expect(screen.getByText('33.3% (95% CI: 13.8%–60.9%)')).toBeInTheDocument();
  expect(screen.getByText('40.0% (95% CI: 19.0%–65.0%)')).toBeInTheDocument();
  expect(
    screen.getByText('This is an observational comparison, not a controlled experiment.'),
  ).toBeInTheDocument();
});

test('a suppressed proportion shows insufficient data instead of a percentage', async () => {
  mockGetEvidence({
    bundle_version: 1,
    generated_at: '2026-09-06T00:00:00Z',
    assessment: baseAssessment,
    observation_window_hours: 72,
    phases: [
      {
        phase: 'baseline',
        campaign_id: 100,
        recognition: {
          numerator: 1,
          denominator: 2,
          rate: 0.5,
          ci_low: 0.09,
          ci_high: 0.91,
          suppressed: true,
        },
      },
    ],
    limitations: [],
  });

  renderWithClient();

  expect(await screen.findByText('Insufficient data (n=2)')).toBeInTheDocument();
  expect(screen.queryByText(/50\.0%/)).not.toBeInTheDocument();
  expect(screen.queryByText(/95% CI/)).not.toBeInTheDocument();
});

test('renders a clear message when no phases are linked yet', async () => {
  mockGetEvidence({
    bundle_version: 1,
    generated_at: '2026-09-06T00:00:00Z',
    assessment: baseAssessment,
    observation_window_hours: 72,
    phases: [],
    limitations: [],
  });

  renderWithClient();

  expect(await screen.findByText('No phases linked yet.')).toBeInTheDocument();
});

test('shows an error state when the evidence fetch fails', async () => {
  vi.spyOn(assessmentsApi, 'getEvidence').mockRejectedValue(new Error('network error'));

  renderWithClient();

  expect(await screen.findByRole('alert')).toHaveTextContent('Could not load evidence');
});
