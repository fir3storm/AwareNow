import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Play, Trash2, Eye } from 'lucide-react';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Table } from '../../components/ui/Table';
import { Modal } from '../../components/ui/Modal';
import { campaignsApi } from '../../api/campaigns';
import type { Campaign } from '../../types';

export function CampaignList() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [deleteModal, setDeleteModal] = useState<{ open: boolean; campaign: Campaign | null }>({
    open: false,
    campaign: null,
  });

  const { data: campaigns, isLoading } = useQuery({
    queryKey: ['campaigns'],
    queryFn: async () => {
      const res = await campaignsApi.getAll();
      return res.data.data;
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => campaignsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['campaigns'] });
      setDeleteModal({ open: false, campaign: null });
    },
  });

  const completeMutation = useMutation({
    mutationFn: (id: number) => campaignsApi.complete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['campaigns'] });
    },
  });

  const columns = [
    {
      key: 'name',
      header: 'Name',
      render: (campaign: Campaign) => (
        <div>
          <p className="font-medium">{campaign.name}</p>
          <p className="text-sm text-gray-500">{campaign.url}</p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (campaign: Campaign) => (
        <span className={`badge ${getStatusBadge(campaign.status)}`}>
          {campaign.status}
        </span>
      ),
    },
    {
      key: 'stats',
      header: 'Results',
      render: (campaign: Campaign) => (
        <div className="text-sm">
          <span className="text-green-600">{campaign.stats?.sent ?? 0} sent</span>
          <span className="mx-1">|</span>
          <span className="text-blue-600">{campaign.stats?.clicked ?? 0} clicked</span>
        </div>
      ),
    },
    {
      key: 'launch_date',
      header: 'Launch Date',
      render: (campaign: Campaign) => (
        <span className="text-sm text-gray-500">
          {new Date(campaign.launch_date).toLocaleDateString()}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      render: (campaign: Campaign) => (
        <div className="flex items-center gap-2 justify-end">
          <button
            onClick={(e) => {
              e.stopPropagation();
              navigate(`/campaigns/${campaign.id}`);
            }}
            className="p-2 text-gray-500 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg transition-colors"
            title="View Results"
          >
            <Eye className="w-4 h-4" />
          </button>
          {campaign.status === 'In Progress' && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                completeMutation.mutate(campaign.id);
              }}
              className="p-2 text-gray-500 hover:text-yellow-600 hover:bg-yellow-50 rounded-lg transition-colors"
              title="Complete Campaign"
            >
              <Play className="w-4 h-4" />
            </button>
          )}
          <button
            onClick={(e) => {
              e.stopPropagation();
              setDeleteModal({ open: true, campaign });
            }}
            className="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
            title="Delete"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Campaigns</h1>
          <p className="text-gray-500 mt-1">Manage your phishing campaigns</p>
        </div>
        <Button onClick={() => navigate('/campaigns/new')}>
          <Plus className="w-4 h-4 mr-2" />
          New Campaign
        </Button>
      </div>

      <Card>
        {isLoading ? (
          <div className="animate-pulse space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-12 bg-gray-100 rounded"></div>
            ))}
          </div>
        ) : (
          <Table
            columns={columns}
            data={campaigns ?? []}
            keyExtractor={(c) => c.id}
            onRowClick={(c) => navigate(`/campaigns/${c.id}`)}
            emptyMessage="No campaigns yet. Create your first campaign to get started."
          />
        )}
      </Card>

      <Modal
        isOpen={deleteModal.open}
        onClose={() => setDeleteModal({ open: false, campaign: null })}
        title="Delete Campaign"
        size="sm"
      >
        <p className="text-gray-600 mb-6">
          Are you sure you want to delete "{deleteModal.campaign?.name}"? This action cannot be undone.
        </p>
        <div className="flex justify-end gap-3">
          <Button
            variant="secondary"
            onClick={() => setDeleteModal({ open: false, campaign: null })}
          >
            Cancel
          </Button>
          <Button
            variant="danger"
            loading={deleteMutation.isPending}
            onClick={() => deleteModal.campaign && deleteMutation.mutate(deleteModal.campaign.id)}
          >
            Delete
          </Button>
        </div>
      </Modal>
    </div>
  );
}

function getStatusBadge(status: string): string {
  switch (status) {
    case 'Completed':
      return 'badge-green';
    case 'In Progress':
      return 'badge-yellow';
    case 'Queued':
      return 'badge-gray';
    case 'Emails Sent':
      return 'badge-green';
    default:
      return 'badge-gray';
  }
}
