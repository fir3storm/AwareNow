import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Edit, Trash2, Send } from 'lucide-react';
import { useState } from 'react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Table } from '../../components/ui/Table';
import { Modal } from '../../components/ui/Modal';
import { smtpApi } from '../../api/smtp';
import type { SMTP } from '../../types';

export function SMTPList() {
  const queryClient = useQueryClient();
  const [deleteModal, setDeleteModal] = useState<{ open: boolean; smtp: SMTP | null }>({
    open: false,
    smtp: null,
  });

  const { data: smtps, isLoading } = useQuery({
    queryKey: ['smtps'],
    queryFn: async () => {
      const res = await smtpApi.getAll();
      return res.data.data;
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => smtpApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['smtps'] });
      setDeleteModal({ open: false, smtp: null });
    },
  });

  const columns = [
    {
      key: 'name',
      header: 'Name',
      render: (smtp: SMTP) => (
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-50 rounded-lg">
            <Send className="w-4 h-4 text-indigo-600" />
          </div>
          <p className="font-medium">{smtp.name}</p>
        </div>
      ),
    },
    {
      key: 'host',
      header: 'SMTP Server',
      render: (smtp: SMTP) => (
        <span className="text-sm text-gray-500">{smtp.host}</span>
      ),
    },
    {
      key: 'from_address',
      header: 'From Address',
      render: (smtp: SMTP) => (
        <span className="text-sm text-gray-500">{smtp.from_address}</span>
      ),
    },
    {
      key: 'modified_date',
      header: 'Last Modified',
      render: (smtp: SMTP) => (
        <span className="text-sm text-gray-500">
          {new Date(smtp.modified_date).toLocaleDateString()}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      render: (smtp: SMTP) => (
        <div className="flex items-center gap-2 justify-end">
          <button
            className="p-2 text-gray-500 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg transition-colors"
            title="Edit"
          >
            <Edit className="w-4 h-4" />
          </button>
          <button
            onClick={(e) => {
              e.stopPropagation();
              setDeleteModal({ open: true, smtp });
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
          <h1 className="text-2xl font-bold text-gray-900">Sending Profiles</h1>
          <p className="text-gray-500 mt-1">Manage your SMTP configurations</p>
        </div>
        <Button>
          <Plus className="w-4 h-4 mr-2" />
          New Profile
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
            data={smtps ?? []}
            keyExtractor={(s) => s.id}
            emptyMessage="No sending profiles yet. Create your first SMTP profile."
          />
        )}
      </Card>

      <Modal
        isOpen={deleteModal.open}
        onClose={() => setDeleteModal({ open: false, smtp: null })}
        title="Delete Sending Profile"
        size="sm"
      >
        <p className="text-gray-600 mb-6">
          Are you sure you want to delete "{deleteModal.smtp?.name}"?
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={() => setDeleteModal({ open: false, smtp: null })}>
            Cancel
          </Button>
          <Button
            variant="danger"
            loading={deleteMutation.isPending}
            onClick={() => deleteModal.smtp && deleteMutation.mutate(deleteModal.smtp.id)}
          >
            Delete
          </Button>
        </div>
      </Modal>
    </div>
  );
}
