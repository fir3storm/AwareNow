import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Edit, Trash2, Users } from 'lucide-react';
import { useState } from 'react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Table } from '../../components/ui/Table';
import { Modal } from '../../components/ui/Modal';
import { groupsApi } from '../../api/groups';
import type { Group } from '../../types';

export function GroupList() {
  const queryClient = useQueryClient();
  const [deleteModal, setDeleteModal] = useState<{ open: boolean; group: Group | null }>({
    open: false,
    group: null,
  });

  const { data: groups, isLoading } = useQuery({
    queryKey: ['groups'],
    queryFn: async () => {
      const res = await groupsApi.getAll();
      return res.data.data;
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => groupsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      setDeleteModal({ open: false, group: null });
    },
  });

  const columns = [
    {
      key: 'name',
      header: 'Name',
      render: (group: Group) => (
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-50 rounded-lg">
            <Users className="w-4 h-4 text-indigo-600" />
          </div>
          <p className="font-medium">{group.name}</p>
        </div>
      ),
    },
    {
      key: 'targets',
      header: 'Targets',
      render: (group: Group) => (
        <span className="text-sm text-gray-500">{group.targets?.length ?? 0} recipients</span>
      ),
    },
    {
      key: 'modified_date',
      header: 'Last Modified',
      render: (group: Group) => (
        <span className="text-sm text-gray-500">
          {new Date(group.modified_date).toLocaleDateString()}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      render: (group: Group) => (
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
              setDeleteModal({ open: true, group });
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
          <h1 className="text-2xl font-bold text-gray-900">Users & Groups</h1>
          <p className="text-gray-500 mt-1">Manage your target groups</p>
        </div>
        <Button>
          <Plus className="w-4 h-4 mr-2" />
          New Group
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
            data={groups ?? []}
            keyExtractor={(g) => g.id}
            emptyMessage="No groups yet. Create your first target group."
          />
        )}
      </Card>

      <Modal
        isOpen={deleteModal.open}
        onClose={() => setDeleteModal({ open: false, group: null })}
        title="Delete Group"
        size="sm"
      >
        <p className="text-gray-600 mb-6">
          Are you sure you want to delete "{deleteModal.group?.name}"?
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={() => setDeleteModal({ open: false, group: null })}>
            Cancel
          </Button>
          <Button
            variant="danger"
            loading={deleteMutation.isPending}
            onClick={() => deleteModal.group && deleteMutation.mutate(deleteModal.group.id)}
          >
            Delete
          </Button>
        </div>
      </Modal>
    </div>
  );
}
