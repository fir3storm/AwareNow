import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Edit, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Table } from '../../components/ui/Table';
import { Modal } from '../../components/ui/Modal';
import { templatesApi } from '../../api/templates';
import type { Template } from '../../types';

export function TemplateList() {
  const queryClient = useQueryClient();
  const [deleteModal, setDeleteModal] = useState<{ open: boolean; template: Template | null }>({
    open: false,
    template: null,
  });

  const { data: templates, isLoading, isError } = useQuery({
    queryKey: ['templates'],
    queryFn: async () => {
      const res = await templatesApi.getAll();
      return res.data;
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => templatesApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['templates'] });
      setDeleteModal({ open: false, template: null });
    },
  });

  const columns = [
    {
      key: 'name',
      header: 'Name',
      render: (template: Template) => (
        <div>
          <p className="font-medium">{template.name}</p>
          <p className="text-sm text-gray-500">{template.subject}</p>
        </div>
      ),
    },
    {
      key: 'modified_date',
      header: 'Last Modified',
      render: (template: Template) => (
        <span className="text-sm text-gray-500">
          {new Date(template.modified_date).toLocaleDateString()}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      render: (template: Template) => (
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
              setDeleteModal({ open: true, template });
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
          <h1 className="text-2xl font-bold text-gray-900">Email Templates</h1>
          <p className="text-gray-500 mt-1">Manage your email templates</p>
        </div>
        <Button>
          <Plus className="w-4 h-4 mr-2" />
          New Template
        </Button>
      </div>

      <Card>
        {isError ? (
          <p role="alert" className="text-red-700">Could not load templates. Refresh the page to try again.</p>
        ) : isLoading ? (
          <div className="animate-pulse space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-12 bg-gray-100 rounded"></div>
            ))}
          </div>
        ) : (
          <Table
            columns={columns}
            data={templates ?? []}
            keyExtractor={(t) => t.id}
            emptyMessage="No templates yet. Create your first email template."
          />
        )}
      </Card>

      <Modal
        isOpen={deleteModal.open}
        onClose={() => setDeleteModal({ open: false, template: null })}
        title="Delete Template"
        size="sm"
      >
        <p className="text-gray-600 mb-6">
          Are you sure you want to delete "{deleteModal.template?.name}"?
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={() => setDeleteModal({ open: false, template: null })}>
            Cancel
          </Button>
          <Button
            variant="danger"
            loading={deleteMutation.isPending}
            onClick={() => deleteModal.template && deleteMutation.mutate(deleteModal.template.id)}
          >
            Delete
          </Button>
        </div>
      </Modal>
    </div>
  );
}
