import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Table } from '../../components/ui/Table';
import { Modal } from '../../components/ui/Modal';
import { reportedMessagesApi } from '../../api/reportedMessages';
import type { ReportedMessage } from '../../api/reportedMessages';

const statusStyles: Record<ReportedMessage['status'], string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  approved: 'bg-green-100 text-green-800',
  rejected: 'bg-gray-100 text-gray-600',
};

export function ReportedMessageList() {
  const queryClient = useQueryClient();
  const [approveModal, setApproveModal] = useState<{ open: boolean; message: ReportedMessage | null }>({
    open: false,
    message: null,
  });
  const [templateName, setTemplateName] = useState('');

  const { data: reportedMessages, isLoading } = useQuery({
    queryKey: ['reportedMessages'],
    queryFn: async () => {
      const res = await reportedMessagesApi.getAll();
      return res.data;
    },
  });

  const approveMutation = useMutation({
    mutationFn: ({ id, name }: { id: number; name: string }) => reportedMessagesApi.approve(id, name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['reportedMessages'] });
      setApproveModal({ open: false, message: null });
    },
  });

  const rejectMutation = useMutation({
    mutationFn: (id: number) => reportedMessagesApi.reject(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['reportedMessages'] });
    },
  });

  const openApproveModal = (message: ReportedMessage) => {
    setTemplateName(`From report: ${message.subject}`);
    setApproveModal({ open: true, message });
  };

  const columns = [
    {
      key: 'reporter_email',
      header: 'Reporter',
      render: (message: ReportedMessage) => (
        <span className="text-sm text-gray-900">{message.reporter_email}</span>
      ),
    },
    {
      key: 'subject',
      header: 'Subject',
      render: (message: ReportedMessage) => (
        <span className="font-medium">{message.subject}</span>
      ),
    },
    {
      key: 'created_at',
      header: 'Reported',
      render: (message: ReportedMessage) => (
        <span className="text-sm text-gray-500">
          {new Date(message.created_at).toLocaleDateString()}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (message: ReportedMessage) => (
        <span
          className={`inline-block px-2 py-1 rounded-full text-xs font-medium capitalize ${statusStyles[message.status]}`}
        >
          {message.status}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      render: (message: ReportedMessage) =>
        message.status === 'pending' ? (
          <div className="flex items-center gap-2 justify-end">
            <Button
              variant="secondary"
              onClick={(e) => {
                e.stopPropagation();
                openApproveModal(message);
              }}
            >
              Approve
            </Button>
            <Button
              variant="danger"
              loading={rejectMutation.isPending && rejectMutation.variables === message.id}
              onClick={(e) => {
                e.stopPropagation();
                rejectMutation.mutate(message.id);
              }}
            >
              Reject
            </Button>
          </div>
        ) : (
          <span className="text-sm text-gray-500 block text-right">
            {message.reviewed_by ? `Reviewed by ${message.reviewed_by}` : ''}
          </span>
        ),
    },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Reported Messages</h1>
          <p className="text-gray-500 mt-1">Review real phishing reports submitted by recipients</p>
        </div>
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
            data={reportedMessages ?? []}
            keyExtractor={(m) => m.id}
            emptyMessage="No reported messages yet."
          />
        )}
      </Card>

      <Modal
        isOpen={approveModal.open}
        onClose={() => setApproveModal({ open: false, message: null })}
        title="Approve Reported Message"
        size="sm"
      >
        <p className="text-gray-600 mb-4">
          Convert "{approveModal.message?.subject}" into a new email template.
        </p>
        <label className="block text-sm font-medium text-gray-700 mb-1">Template name</label>
        <input
          type="text"
          value={templateName}
          onChange={(e) => setTemplateName(e.target.value)}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg mb-6 focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={() => setApproveModal({ open: false, message: null })}>
            Cancel
          </Button>
          <Button
            loading={approveMutation.isPending}
            onClick={() =>
              approveModal.message &&
              approveMutation.mutate({ id: approveModal.message.id, name: templateName })
            }
          >
            Approve
          </Button>
        </div>
      </Modal>
    </div>
  );
}
