import type { AwarenessMetrics } from './types';
import './controlPlane.css';

interface AwarenessOverviewProps {
  metrics: AwarenessMetrics;
}

const metricLabels: Array<{ key: keyof AwarenessMetrics; label: string }> = [
  { key: 'sent', label: 'Messages sent' },
  { key: 'opened', label: 'Messages opened' },
  { key: 'clicked', label: 'Links clicked' },
  { key: 'reported', label: 'Reports received' },
  { key: 'trainingCompleted', label: 'Training completed' },
];

const numberFormatter = new Intl.NumberFormat('en-US');

export function AwarenessOverview({ metrics }: AwarenessOverviewProps) {
  return (
    <section className="control-plane-section" aria-labelledby="awareness-overview-title">
      <div className="control-plane-section-heading">
        <div>
          <p className="control-plane-eyebrow">Awareness outcomes</p>
          <h2 id="awareness-overview-title">Program overview</h2>
        </div>
        <p>Aggregate activity for this tenant only.</p>
      </div>
      <dl className="control-plane-metric-grid">
        {metricLabels.map(({ key, label }) => (
          <div className="control-plane-metric" key={key}>
            <dt>{label}</dt>
            <dd>{numberFormatter.format(metrics[key])}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
