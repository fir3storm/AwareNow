import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, test, vi } from 'vitest';
import { ReportedMessageList } from './ReportedMessageList';
import { reportedMessagesApi } from '../../api/reportedMessages';
import type { ReportedMessage } from '../../api/reportedMessages';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function renderWithClient() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <ReportedMessageList />
    </QueryClientProvider>
  );
}

function mockGetAll(messages: ReportedMessage[]) {
  vi.spyOn(reportedMessagesApi, 'getAll').mockResolvedValue({
    data: messages,
  } as Awaited<ReturnType<typeof reportedMessagesApi.getAll>>);
}

test('renders a pending reported message with Approve/Reject actions', async () => {
  mockGetAll([
    {
      id: 1,
      reporter_email: 'alice@example.com',
      subject: 'Urgent: verify your account',
      body_text: '',
      body_html: '',
      status: 'pending',
      converted_template_id: 0,
      reviewed_by: '',
      created_at: '2026-09-05T00:00:00Z',
      reviewed_at: '',
    },
  ]);

  renderWithClient();

  expect(await screen.findByText('Urgent: verify your account')).toBeInTheDocument();
  expect(screen.getByText('alice@example.com')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /approve/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /reject/i })).toBeInTheDocument();
});

test('does not show actions for an already-reviewed message', async () => {
  mockGetAll([
    {
      id: 2,
      reporter_email: 'bob@example.com',
      subject: 'Reviewed report',
      body_text: '',
      body_html: '',
      status: 'approved',
      converted_template_id: 5,
      reviewed_by: 'admin',
      created_at: '2026-09-05T00:00:00Z',
      reviewed_at: '2026-09-05T01:00:00Z',
    },
  ]);

  renderWithClient();

  expect(await screen.findByText('Reviewed report')).toBeInTheDocument();
  expect(screen.getByText('Reviewed by admin')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /approve/i })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /reject/i })).not.toBeInTheDocument();
});

test('renders the empty state when there are no reported messages', async () => {
  mockGetAll([]);

  renderWithClient();

  expect(await screen.findByText('No reported messages yet.')).toBeInTheDocument();
});
