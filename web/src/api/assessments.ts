import client from './client';

export interface Assessment {
  id: number;
  name: string;
  skill_tag: string;
  baseline_scenario_id: number;
  followup_scenario_id: number;
  benign_scenario_id: number;
  observation_window_hours: number;
  status: string;
  created_at: string;
}

// Proportion mirrors assessment.Proportion (assessment/metrics.go), which
// now carries explicit snake_case json tags matching the rest of this API.
export interface Proportion {
  numerator: number;
  denominator: number;
  rate: number;
  ci_low: number;
  ci_high: number;
  suppressed: boolean;
}

// SpeedResult mirrors assessment.SpeedResult. median_ns/p25_ns/p75_ns are
// Go time.Duration values serialized as a plain number of nanoseconds (the
// _ns suffix is explicit in the API contract) — convert before displaying.
export interface SpeedResult {
  eligible: number;
  any_report_count: number;
  nonreporting: Proportion;
  median_ns: number;
  p25_ns: number;
  p75_ns: number;
}

export type AssessmentPhaseKind = 'baseline' | 'followup' | 'benign_control';

export interface PhaseEvidence {
  phase: AssessmentPhaseKind | string;
  campaign_id: number;
  recognition?: Proportion;
  recovery?: Proportion;
  speed?: SpeedResult;
  discrimination?: Proportion;
}

export interface EvidenceBundle {
  bundle_version: number;
  generated_at: string;
  assessment: Assessment;
  observation_window_hours: number;
  phases: PhaseEvidence[];
  limitations: string[];
}

export type EvidenceExportFormat = 'pdf' | 'xlsx';

export const assessmentsApi = {
  getAll: () => client.get<Assessment[]>('/assessments/'),

  getById: (id: number) => client.get<Assessment>(`/assessments/${id}`),

  getEvidence: (id: number) =>
    client.get<EvidenceBundle>(`/assessments/${id}/evidence`, { params: { format: 'json' } }),

  exportEvidence: (id: number, format: EvidenceExportFormat) =>
    client.get<Blob>(`/assessments/${id}/evidence`, {
      params: { format },
      responseType: 'blob',
    }),
};
