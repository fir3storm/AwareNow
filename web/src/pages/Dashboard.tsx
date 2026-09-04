import { useQuery } from '@tanstack/react-query';
import { Mail, MousePointer, Eye, AlertTriangle, TrendingUp, Users } from 'lucide-react';
import { Card, StatCard } from '../components/ui/Card';
import { analyticsApi } from '../api/analytics';
import { campaignsApi } from '../api/campaigns';

export function Dashboard() {
  const { data: overview, isLoading: overviewLoading } = useQuery({
    queryKey: ['analytics-overview'],
    queryFn: async () => {
      const res = await analyticsApi.getOverview();
      return res.data.data;
    },
  });

  const { data: campaigns, isLoading: campaignsLoading } = useQuery({
    queryKey: ['campaigns-summary'],
    queryFn: async () => {
      const res = await campaignsApi.getSummary();
      return res.data.data.campaigns;
    },
  });

  const isLoading = overviewLoading || campaignsLoading;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-gray-500 mt-1">Security awareness overview</p>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {[...Array(4)].map((_, i) => (
            <Card key={i}>
              <div className="animate-pulse">
                <div className="h-4 bg-gray-200 rounded w-1/2 mb-2"></div>
                <div className="h-8 bg-gray-200 rounded w-1/3"></div>
              </div>
            </Card>
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <StatCard
            title="Total Campaigns"
            value={overview?.total_campaigns ?? 0}
            icon={<Mail className="w-5 h-5" />}
            color="indigo"
          />
          <StatCard
            title="Emails Sent"
            value={overview?.emails_sent ?? 0}
            icon={<TrendingUp className="w-5 h-5" />}
            color="green"
          />
          <StatCard
            title="Click Rate"
            value={`${((overview?.click_rate ?? 0) * 100).toFixed(1)}%`}
            icon={<MousePointer className="w-5 h-5" />}
            color="yellow"
          />
          <StatCard
            title="Open Rate"
            value={`${((overview?.open_rate ?? 0) * 100).toFixed(1)}%`}
            icon={<Eye className="w-5 h-5" />}
            color="indigo"
          />
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Risk Score</h3>
          <div className="flex items-center justify-center py-8">
            <div className="relative">
              <svg className="w-32 h-32 transform -rotate-90">
                <circle
                  cx="64"
                  cy="64"
                  r="56"
                  stroke="#e5e7eb"
                  strokeWidth="12"
                  fill="none"
                />
                <circle
                  cx="64"
                  cy="64"
                  r="56"
                  stroke={getRiskColor(overview?.risk_score ?? 0)}
                  strokeWidth="12"
                  fill="none"
                  strokeDasharray={`${(overview?.risk_score ?? 0) * 3.52} 352`}
                  strokeLinecap="round"
                />
              </svg>
              <div className="absolute inset-0 flex items-center justify-center">
                <span className="text-3xl font-bold text-gray-900">
                  {overview?.risk_score ?? 0}
                </span>
              </div>
            </div>
          </div>
          <p className="text-center text-sm text-gray-500">
            {getRiskLabel(overview?.risk_score ?? 0)}
          </p>
        </Card>

        <Card>
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Recent Campaigns</h3>
          {campaigns && campaigns.length > 0 ? (
            <div className="space-y-3">
              {campaigns.slice(0, 5).map((campaign) => (
                <div
                  key={campaign.id}
                  className="flex items-center justify-between p-3 bg-gray-50 rounded-lg"
                >
                  <div>
                    <p className="font-medium text-gray-900">{campaign.name}</p>
                    <p className="text-sm text-gray-500">{campaign.status}</p>
                  </div>
                  <span className={`badge ${getStatusBadge(campaign.status)}`}>
                    {campaign.stats.clicked ?? 0} clicks
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-gray-500 text-center py-8">No campaigns yet</p>
          )}
        </Card>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <StatCard
          title="Submit Rate"
          value={`${((overview?.submit_rate ?? 0) * 100).toFixed(1)}%`}
          icon={<Users className="w-5 h-5" />}
          color="red"
        />
        <StatCard
          title="Report Rate"
          value={`${((overview?.report_rate ?? 0) * 100).toFixed(1)}%`}
          icon={<AlertTriangle className="w-5 h-5" />}
          color="green"
        />
        <StatCard
          title="Avg Time to Click"
          value={overview?.avg_time_to_click ?? 'N/A'}
          icon={<TrendingUp className="w-5 h-5" />}
          color="indigo"
        />
      </div>
    </div>
  );
}

function getRiskColor(score: number): string {
  if (score >= 70) return '#ef4444';
  if (score >= 40) return '#f59e0b';
  return '#22c55e';
}

function getRiskLabel(score: number): string {
  if (score >= 70) return 'High Risk - Immediate attention needed';
  if (score >= 40) return 'Medium Risk - Improvement recommended';
  return 'Low Risk - Good security awareness';
}

function getStatusBadge(status: string): string {
  switch (status) {
    case 'Completed':
      return 'badge-green';
    case 'In Progress':
      return 'badge-yellow';
    case 'Queued':
      return 'badge-gray';
    default:
      return 'badge-gray';
  }
}
