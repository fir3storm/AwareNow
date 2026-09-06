import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Table } from '../../components/ui/Table';
import { Modal } from '../../components/ui/Modal';
import { reportedMessagesApi } from '../../api/reportedMessages';
import type { ReportedMessage, ReportedMessageStatus } from '../../api/reportedMessages';

const statusStyles: Record<ReportedMessage['status'], string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  approved: 'bg-green-100 text-green-800',
  rejected: 'bg-gray-100 text-gray-600',
};

const PER_PAGE = 20;
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Wraps untrusted HTML in a minimal document with a strict Content-Security-
 * Policy so it can be safely rendered inside a sandboxed iframe (sandbox="").
 * The CSP blocks all network loads except inline data: images, and the empty
 * sandbox attribute (set on the iframe itself) blocks script execution and
 * any navigation originating from the frame.
 */
function buildSafeSrcDoc(html: string): string {
  const csp =
    '<meta http-equiv="Content-Security-Policy" content="default-src \'none\'; img-src data:; style-src \'unsafe-inline\'">';
  return `${csp}${html}`;
}

function toStartOfDayISO(dateStr: string): string {
  return new Date(`${dateStr}T00:00:00.000Z`).toISOString();
}

function toEndOfDayISO(dateStr: string): string {
  return new Date(`${dateStr}T23:59:59.999Z`).toISOString();
}

export function ReportedMessageList() {
  const queryClient = useQueryClient();
  const [approveModal, setApproveModal] = useState<{ open: boolean; message: ReportedMessage | null }>({
    open: false,
    message: null,
  });
  const [templateName, setTemplateName] = useState('');

  const [detailMessage, setDetailMessage] = useState<ReportedMessage | null>(null);
  const [viewAsHtml, setViewAsHtml] = useState(false);

  const [status, setStatus] = useState<ReportedMessageStatus | ''>('');
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [page, setPage] = useState(1);

  // Debounce the free-text search so we don't refetch on every keystroke.
  // Reset to page 1 once the debounced value actually changes so the user
  // isn't stranded on a now-empty page of a newly-filtered result set.
  useEffect(() => {
    const handle = setTimeout(() => {
      setSearch(searchInput);
      setPage(1);
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(handle);
  }, [searchInput]);

  const createdAfter = dateFrom ? toStartOfDayISO(dateFrom) : undefined;
  const createdBefore = dateTo ? toEndOfDayISO(dateTo) : undefined;

  const queryParams = {
    status: status || undefined,
    search: search || undefined,
    created_after: createdAfter,
    created_before: createdBefore,
    page,
    per_page: PER_PAGE,
  };

  const { data, isLoading, isFetching, isError } = useQuery({
    queryKey: ['reportedMessages', { status, search, createdAfter, createdBefore, page }],
    queryFn: async () => {
      const res = await reportedMessagesApi.getAll(queryParams);
      return res.data;
    },
    placeholderData: (previousData) => previousData,
  });

  const reportedMessages = data?.data ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));

  const approveMutation = useMutation({
    mutationFn: ({ id, name }: { id: number; name: string }) => reportedMessagesApi.approve(id, name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['reportedMessages'] });
      queryClient.invalidateQueries({ queryKey: ['templates'] });
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
    approveMutation.reset();
    setTemplateName(`From report: ${message.subject}`);
    setApproveModal({ open: true, message });
  };

  const openDetailModal = (message: ReportedMessage) => {
    setViewAsHtml(false);
    setDetailMessage(message);
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

      {rejectMutation.isError && (
        <p role="alert" className="text-red-700">Could not reject this report. Refresh the queue and try again.</p>
      )}

      <Card>
        <div className="flex flex-wrap items-end gap-4 mb-4">
          <div>
            <label htmlFor="report-status-filter" className="block text-sm font-medium text-gray-700 mb-1">
              Status
            </label>
            <select
              id="report-status-filter"
              value={status}
              onChange={(e) => {
                setStatus(e.target.value as ReportedMessageStatus | '');
                setPage(1);
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
            >
              <option value="">All</option>
              <option value="pending">Pending</option>
              <option value="approved">Approved</option>
              <option value="rejected">Rejected</option>
            </select>
          </div>
          <div>
            <label htmlFor="report-search" className="block text-sm font-medium text-gray-700 mb-1">
              Search
            </label>
            <input
              id="report-search"
              type="text"
              placeholder="Reporter email or subject"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
            />
          </div>
          <div>
            <label htmlFor="report-date-from" className="block text-sm font-medium text-gray-700 mb-1">
              From
            </label>
            <input
              id="report-date-from"
              type="date"
              value={dateFrom}
              onChange={(e) => {
                setDateFrom(e.target.value);
                setPage(1);
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
            />
          </div>
          <div>
            <label htmlFor="report-date-to" className="block text-sm font-medium text-gray-700 mb-1">
              To
            </label>
            <input
              id="report-date-to"
              type="date"
              value={dateTo}
              onChange={(e) => {
                setDateTo(e.target.value);
                setPage(1);
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
            />
          </div>
          {isFetching && !isLoading && (
            <span className="text-sm text-gray-400">Updating...</span>
          )}
        </div>

        {isError ? (
          <p role="alert" className="text-red-700">Could not load reported messages. Refresh the page to try again.</p>
        ) : isLoading ? (
          <div className="animate-pulse space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-12 bg-gray-100 rounded"></div>
            ))}
          </div>
        ) : (
          <>
            <Table
              columns={columns}
              data={reportedMessages}
              keyExtractor={(m) => m.id}
              onRowClick={openDetailModal}
              emptyMessage="No reported messages yet."
            />
            {reportedMessages.length > 0 && (
              <div className="flex items-center justify-between mt-4">
                <span className="text-sm text-gray-500">
                  Page {page} of {totalPages}
                </span>
                <div className="flex gap-2">
                  <Button
                    variant="secondary"
                    disabled={page <= 1}
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="secondary"
                    disabled={page >= totalPages}
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
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
        <label htmlFor="report-template-name" className="block text-sm font-medium text-gray-700 mb-1">Template name</label>
        <input
          id="report-template-name"
          type="text"
          value={templateName}
          onChange={(e) => setTemplateName(e.target.value)}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg mb-6 focus:outline-none focus:ring-2 focus:ring-indigo-500"
        />
        {approveMutation.isError && (
          <p role="alert" className="text-red-700 mb-4">Could not approve this report. Check the template name or refresh the queue before trying again.</p>
        )}
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

      <Modal
        isOpen={detailMessage !== null}
        onClose={() => setDetailMessage(null)}
        title="Reported Message Detail"
        size="lg"
      >
        {detailMessage && (
          <div className="space-y-4">
            <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
              <dt className="text-gray-500">Reporter</dt>
              <dd className="text-gray-900">{detailMessage.reporter_email}</dd>
              <dt className="text-gray-500">Subject</dt>
              <dd className="text-gray-900">{detailMessage.subject}</dd>
              <dt className="text-gray-500">Status</dt>
              <dd>
                <span
                  className={`inline-block px-2 py-1 rounded-full text-xs font-medium capitalize ${statusStyles[detailMessage.status]}`}
                >
                  {detailMessage.status}
                </span>
              </dd>
              <dt className="text-gray-500">Reported at</dt>
              <dd className="text-gray-900">{new Date(detailMessage.created_at).toLocaleString()}</dd>
              <dt className="text-gray-500">Reviewed by</dt>
              <dd className="text-gray-900">{detailMessage.reviewed_by || '-'}</dd>
              <dt className="text-gray-500">Reviewed at</dt>
              <dd className="text-gray-900">
                {detailMessage.reviewed_at ? new Date(detailMessage.reviewed_at).toLocaleString() : '-'}
              </dd>
            </dl>

            <div>
              <div className="flex items-center justify-between mb-2">
                <h3 className="text-sm font-medium text-gray-700">Message body</h3>
                {detailMessage.body_html && (
                  <label className="flex items-center gap-2 text-sm text-gray-600">
                    <input
                      type="checkbox"
                      checked={viewAsHtml}
                      onChange={(e) => setViewAsHtml(e.target.checked)}
                    />
                    View as HTML
                  </label>
                )}
              </div>

              {viewAsHtml && detailMessage.body_html ? (
                <>
                  <p className="text-xs text-gray-500 mb-2">
                    HTML preview is sandboxed: scripts, images, and links are disabled.
                  </p>
                  <iframe
                    title="Reported message HTML preview"
                    srcDoc={buildSafeSrcDoc(detailMessage.body_html)}
                    sandbox=""
                    className="w-full h-80 border border-gray-200 rounded overflow-auto"
                  />
                </>
              ) : (
                <pre className="whitespace-pre-wrap text-sm text-gray-900 bg-gray-50 border border-gray-200 rounded p-3 max-h-80 overflow-auto">
                  {detailMessage.body_text}
                </pre>
              )}
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
