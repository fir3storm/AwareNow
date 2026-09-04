import { useQuery } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, Mail, MousePointer, Eye, AlertTriangle, Download } from 'lucide-react';
import { Card, StatCard } from '../../components/ui/Card';
import { Table } from '../../components/ui/Table';
import { Button } from '../../components/ui/Button';
import { campaignsApi } from '../../api/campaigns';
import type { CampaignResult } from '../../types';

export function CampaignResults() {
  const { id } = useParams<{ id: string }>();

  const { data: campaign } = useQuery({
    queryKey: ['campaign', id],
    queryFn: async () => {
      const res = await campaignsApi.getById(Number(id));
      return res.data.data;
    },
  });

  const { data: results } = useQuery({
    queryKey: ['campaign-results', id],
    queryFn: async () => {
      const res = await campaignsApi.getResults(Number(id));
      return res.data.data;
    },
  });

  const columns = [
    {
      key: 'email',
      header: 'Recipient',
      render: (result: CampaignResult) => (
        <div>
          <p className="font-medium">{result.email}</p>
          <p className="text-sm text-gray-500">
            {result.first_name} {result.last_name}
          </p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (result: CampaignResult) => (
        <span className={`badge ${getStatusBadge(result.status)}`}>
          {result.status}
        </span>
      ),
    },
    {
      key: 'ip',
      header: 'IP Address',
      render: (result: CampaignResult) => (
        <span className="text-sm text-gray-500">{result.ip || '-'}</span>
      ),
    },
    {
      key: 'send_date',
      header: 'Last Activity',
      render: (result: CampaignResult) => (
        <span className="text-sm text-gray-500">
          {new Date(result.send_date).toLocaleString()}
        </span>
      ),
    },
    {
      key: 'reported',
      header: 'Reported',
      render: (result: CampaignResult) => (
        result.reported ? (
          <span className="badge-green">Yes</span>
        ) : (
          <span className="badge-gray">No</span>
        )
      ),
    },
  ];

  if (!campaign) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-pulse text-gray-500">Loading...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/campaigns" className="p-2 hover:bg-gray-100 rounded-lg transition-colors">
          <ArrowLeft className="w-5 h-5 text-gray-600" />
        </Link>
        <div className="flex-1">
          <h1 className="text-2xl font-bold text-gray-900">{campaign.name}</h1>
          <p className="text-gray-500 mt-1">Campaign results and analytics</p>
        </div>
        <Button variant="secondary">
          <Download className="w-4 h-4 mr-2" />
          Export
        </Button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Emails Sent"
          value={campaign.stats?.sent ?? 0}
          icon={<Mail className="w-5 h-5" />}
          color="indigo"
        />
        <StatCard
          title="Opened"
          value={campaign.stats?.opened ?? 0}
          icon={<Eye className="w-5 h-5" />}
          color="green"
        />
        <StatCard
          title="Clicked"
          value={campaign.stats?.clicked ?? 0}
          icon={<MousePointer className="w-5 h-5" />}
          color="yellow"
        />
        <StatCard
          title="Reported"
          value={campaign.stats?.reported ?? 0}
          icon={<AlertTriangle className="w-5 h-5" />}
          color="green"
        />
      </div>

      <Card>
        <h3 className="text-lg font-semibold text-gray-900 mb-4">Recipient Activity</h3>
        <Table
          columns={columns}
          data={results ?? []}
          keyExtractor={(r) => r.id}
          emptyMessage="No results yet"
        />
      </Card>
    </div>
  );
}

function getStatusBadge(status: string): string {
  switch (status) {
    case 'Email Sent':
      return 'badge-gray';
    case 'Email Opened':
      return 'badge-yellow';
    case 'Clicked Link':
      return 'badge-red';
    case 'Submitted Data':
      return 'badge-red';
    case 'Reported':
      return 'badge-green';
    case 'Error':
      return 'badge-red';
    default:
      return 'badge-gray';
  }
}
