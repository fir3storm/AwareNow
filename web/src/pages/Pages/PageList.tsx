import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Edit, Trash2, Globe } from 'lucide-react';
import { useState } from 'react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Table } from '../../components/ui/Table';
import { Modal } from '../../components/ui/Modal';
import { pagesApi } from '../../api/pages';
import type { Page } from '../../types';

export function PageList() {
  const queryClient = useQueryClient();
  const [deleteModal, setDeleteModal] = useState<{ open: boolean; page: Page | null }>({
    open: false,
    page: null,
  });

  const { data: pages, isLoading } = useQuery({
    queryKey: ['pages'],
    queryFn: async () => {
      const res = await pagesApi.getAll();
      return res.data.data;
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => pagesApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['pages'] });
      setDeleteModal({ open: false, page: null });
    },
  });

  const columns = [
    {
      key: 'name',
      header: 'Name',
      render: (page: Page) => (
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-50 rounded-lg">
            <Globe className="w-4 h-4 text-indigo-600" />
          </div>
          <p className="font-medium">{page.name}</p>
        </div>
      ),
    },
    {
      key: 'capture_credentials',
      header: 'Captures',
      render: (page: Page) => (
        <div className="flex gap-2">
          {page.capture_credentials && <span className="badge-gray">Credentials</span>}
          {page.capture_passwords && <span className="badge-gray">Passwords</span>}
        </div>
      ),
    },
    {
      key: 'modified_date',
      header: 'Last Modified',
      render: (page: Page) => (
        <span className="text-sm text-gray-500">
          {new Date(page.modified_date).toLocaleDateString()}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      render: (page: Page) => (
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
              setDeleteModal({ open: true, page });
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
          <h1 className="text-2xl font-bold text-gray-900">Landing Pages</h1>
          <p className="text-gray-500 mt-1">Manage your phishing landing pages</p>
        </div>
        <Button>
          <Plus className="w-4 h-4 mr-2" />
          New Page
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
            data={pages ?? []}
            keyExtractor={(p) => p.id}
            emptyMessage="No landing pages yet. Create your first page."
          />
        )}
      </Card>

      <Modal
        isOpen={deleteModal.open}
        onClose={() => setDeleteModal({ open: false, page: null })}
        title="Delete Landing Page"
        size="sm"
      >
        <p className="text-gray-600 mb-6">
          Are you sure you want to delete "{deleteModal.page?.name}"?
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={() => setDeleteModal({ open: false, page: null })}>
            Cancel
          </Button>
          <Button
            variant="danger"
            loading={deleteMutation.isPending}
            onClick={() => deleteModal.page && deleteMutation.mutate(deleteModal.page.id)}
          >
            Delete
          </Button>
        </div>
      </Modal>
    </div>
  );
}
