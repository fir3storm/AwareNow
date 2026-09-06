import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, test, vi } from 'vitest';
import { ReportedMessageList } from './ReportedMessageList';
import { reportedMessagesApi } from '../../api/reportedMessages';
import type { ReportedMessage, ReportedMessageListParams } from '../../api/reportedMessages';

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

function mockGetAll(
  messages: ReportedMessage[],
  meta: { total?: number; page?: number; per_page?: number } = {},
) {
  return vi.spyOn(reportedMessagesApi, 'getAll').mockResolvedValue({
    data: {
      data: messages,
      total: meta.total ?? messages.length,
      page: meta.page ?? 1,
      per_page: meta.per_page ?? 20,
    },
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

test('changing the status filter triggers a new getAll call with the filter applied', async () => {
  const spy = mockGetAll([
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
  await screen.findByText('Urgent: verify your account');

  fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'approved' } });

  await vi.waitFor(() => {
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1]?.[0] as ReportedMessageListParams | undefined;
    expect(lastCall?.status).toBe('approved');
  });
});

test('pagination controls request the next page', async () => {
  const messages: ReportedMessage[] = Array.from({ length: 20 }, (_, i) => ({
    id: i + 1,
    reporter_email: `user${i}@example.com`,
    subject: `Report ${i}`,
    body_text: '',
    body_html: '',
    status: 'pending',
    converted_template_id: 0,
    reviewed_by: '',
    created_at: '2026-09-05T00:00:00Z',
    reviewed_at: '',
  }));
  const spy = mockGetAll(messages, { total: 50, page: 1, per_page: 20 });

  renderWithClient();
  await screen.findByText('Report 0');

  expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();
  const nextButton = screen.getByRole('button', { name: 'Next' });
  expect(nextButton).not.toBeDisabled();
  expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled();

  fireEvent.click(nextButton);

  await vi.waitFor(() => {
    const lastCall = spy.mock.calls[spy.mock.calls.length - 1]?.[0] as ReportedMessageListParams | undefined;
    expect(lastCall?.page).toBe(2);
  });
});

test('safe HTML preview renders a sandboxed iframe with a strict CSP and no live script execution', async () => {
  const maliciousHtml = '<script>window.__pwned = true</script><img src="http://evil.example/track.png"><p>Hello</p>';
  mockGetAll([
    {
      id: 1,
      reporter_email: 'alice@example.com',
      subject: 'Malicious report',
      body_text: 'Hello',
      body_html: maliciousHtml,
      status: 'pending',
      converted_template_id: 0,
      reviewed_by: '',
      created_at: '2026-09-05T00:00:00Z',
      reviewed_at: '',
    },
  ]);

  renderWithClient();
  const row = await screen.findByText('Malicious report');
  fireEvent.click(row);

  expect(await screen.findByText('Reported Message Detail')).toBeInTheDocument();
  // Default view is plain text - React escapes it, so no HTML is parsed.
  expect(screen.getByText(/Hello/)).toBeInTheDocument();

  const toggle = screen.getByLabelText('View as HTML');
  fireEvent.click(toggle);

  const iframe = await screen.findByTitle('Reported message HTML preview');
  // The empty sandbox attribute is the enforcement mechanism: no allow-scripts,
  // no allow-same-origin, no allow-top-navigation, no allow-popups.
  expect(iframe).toHaveAttribute('sandbox', '');
  expect(iframe.getAttribute('srcdoc')).toContain('Content-Security-Policy');
  expect(iframe.getAttribute('srcdoc')).toContain(maliciousHtml);

  // The script must never execute in the real page context.
  expect((window as unknown as { __pwned?: boolean }).__pwned).toBeUndefined();
});
