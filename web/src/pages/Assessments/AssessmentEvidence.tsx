import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { assessmentsApi } from '../../api/assessments';
import type { EvidenceExportFormat, PhaseEvidence, Proportion } from '../../api/assessments';

const phaseLabels: Record<string, string> = {
  baseline: 'Baseline',
  followup: 'Follow-up',
  benign_control: 'Benign Control',
};

function phaseLabel(phase: string): string {
  return phaseLabels[phase] || phase;
}

// Nanoseconds (as returned by Go's time.Duration over JSON) converted into
// a human-readable unit. Picks minutes under an hour and hours otherwise,
// since assessment observation windows and reporting speeds are typically
// measured in hours but a fast phase could be a matter of minutes.
function formatDuration(nanoseconds: number): string {
  const ms = nanoseconds / 1e6;
  const seconds = ms / 1000;
  const minutes = seconds / 60;
  const hours = minutes / 60;
  if (hours >= 1) {
    return `${hours.toFixed(1)}h`;
  }
  if (minutes >= 1) {
    return `${minutes.toFixed(1)}m`;
  }
  return `${seconds.toFixed(1)}s`;
}

// Mirrors controllers/api/assessment.go's proportionLine helper: a
// suppressed proportion (too small a cohort) must never be shown as a
// percentage, only as "Insufficient data (n=<denominator>)".
function formatProportion(p: Proportion): string {
  if (p.suppressed) {
    return `Insufficient data (n=${p.denominator})`;
  }
  return `${(p.rate * 100).toFixed(1)}% (95% CI: ${(p.ci_low * 100).toFixed(1)}%–${(p.ci_high * 100).toFixed(1)}%)`;
}

function MetricRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between py-2 border-b border-gray-100 last:border-0">
      <span className="text-sm text-gray-500">{label}</span>
      <span className="text-sm font-medium text-gray-900">{value}</span>
    </div>
  );
}

function PhaseCard({ phase }: { phase: PhaseEvidence }) {
  return (
    <Card>
      <h3 className="text-lg font-semibold text-gray-900 mb-2">{phaseLabel(phase.phase)}</h3>
      <p className="text-xs text-gray-500 mb-4">Campaign #{phase.campaign_id}</p>

      <div>
        {phase.recognition && <MetricRow label="Recognition" value={formatProportion(phase.recognition)} />}
        {phase.recovery && <MetricRow label="Recovery" value={formatProportion(phase.recovery)} />}
        {phase.speed && (
          <>
            <MetricRow
              label="Speed (median)"
              value={phase.speed.eligible > 0 ? formatDuration(phase.speed.median_ns) : 'N/A'}
            />
            <MetricRow
              label="Speed (P25 / P75)"
              value={
                phase.speed.eligible > 0
                  ? `${formatDuration(phase.speed.p25_ns)} / ${formatDuration(phase.speed.p75_ns)}`
                  : 'N/A'
              }
            />
            <MetricRow label="Nonreporting" value={formatProportion(phase.speed.nonreporting)} />
          </>
        )}
        {phase.discrimination && (
          <MetricRow label="Discrimination" value={formatProportion(phase.discrimination)} />
        )}
        {!phase.recognition && !phase.recovery && !phase.speed && !phase.discrimination && (
          <p className="text-sm text-gray-500">No metrics available for this phase.</p>
        )}
      </div>
    </Card>
  );
}

export function AssessmentEvidence() {
  const { id } = useParams<{ id: string }>();
  const [exportingFormat, setExportingFormat] = useState<EvidenceExportFormat | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const { data: bundle, isLoading, isError } = useQuery({
    queryKey: ['assessment-evidence', id],
    queryFn: async () => {
      const res = await assessmentsApi.getEvidence(Number(id));
      return res.data;
    },
    enabled: !!id,
  });

  const handleExport = async (format: EvidenceExportFormat) => {
    if (!id) return;
    setExportError(null);
    setExportingFormat(format);
    try {
      const res = await assessmentsApi.exportEvidence(Number(id), format);
      const url = URL.createObjectURL(res.data);
      const link = document.createElement('a');
      link.href = url;
      link.download = `assessment-${id}-evidence.${format}`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch {
      setExportError(`Could not export evidence as ${format.toUpperCase()}. Please try again later.`);
    } finally {
      setExportingFormat(null);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <Link to="/assessments" className="inline-flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-700 mb-2">
          <ArrowLeft className="w-4 h-4" />
          Back to Assessments
        </Link>
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">
              {bundle ? bundle.assessment.name : 'Assessment Evidence'}
            </h1>
            {bundle && (
              <p className="text-gray-500 mt-1">
                Skill: {bundle.assessment.skill_tag} &middot; Observation window:{' '}
                {bundle.observation_window_hours}h &middot; Generated{' '}
                {new Date(bundle.generated_at).toLocaleString()}
              </p>
            )}
          </div>

          {bundle && (
            <div className="flex items-center gap-2">
              <span className="text-sm text-gray-500 mr-1">Export:</span>
              {(['pdf', 'xlsx'] as EvidenceExportFormat[]).map((format) => (
                <Button
                  key={format}
                  variant="secondary"
                  size="sm"
                  loading={exportingFormat === format}
                  disabled={exportingFormat !== null}
                  onClick={() => handleExport(format)}
                >
                  {format.toUpperCase()}
                </Button>
              ))}
            </div>
          )}
        </div>
      </div>

      {exportError && (
        <p role="alert" className="text-red-700">
          {exportError}
        </p>
      )}

      {isError ? (
        <p role="alert" className="text-red-700">
          Could not load evidence for this assessment. It may not exist, or you may not have access.
        </p>
      ) : isLoading ? (
        <div className="animate-pulse space-y-4">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="h-24 bg-gray-100 rounded"></div>
          ))}
        </div>
      ) : bundle ? (
        <>
          {bundle.phases.length === 0 ? (
            <Card>
              <p className="text-gray-500 text-center py-8">No phases linked yet.</p>
            </Card>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {bundle.phases.map((phase) => (
                <PhaseCard key={`${phase.phase}-${phase.campaign_id}`} phase={phase} />
              ))}
            </div>
          )}

          <Card>
            <h3 className="text-lg font-semibold text-gray-900 mb-3">Limitations</h3>
            <ul className="list-disc list-inside space-y-2 text-sm text-gray-700">
              {bundle.limitations.map((limitation, i) => (
                <li key={i}>{limitation}</li>
              ))}
            </ul>
          </Card>
        </>
      ) : null}
    </div>
  );
}
