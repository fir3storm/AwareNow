import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { AxiosError, type AxiosAdapter } from 'axios';
import { afterEach, expect, test } from 'vitest';
import client from '../../api/client';
import { templatesApi } from '../../api/templates';
import { ReportedMessageList } from './ReportedMessageList';
import { TemplateList } from '../Templates/TemplateList';
import { CampaignCreate } from '../Campaigns/CampaignCreate';

const originalAdapter = client.defaults.adapter;

afterEach(() => {
  cleanup();
  client.defaults.adapter = originalAdapter;
});

const report = {
  id: 1,
  reporter_email: 'alice@example.com',
  subject: 'Suspicious invoice',
  body_text: 'Review invoice',
  body_html: '',
  status: 'pending',
  converted_template_id: 0,
  reviewed_by: '',
  created_at: '2026-09-05T00:00:00Z',
  reviewed_at: '',
};

const template = {
  id: 42,
  name: 'Invoice assessment',
  subject: report.subject,
  text: report.body_text,
  html: '',
  modified_date: '2026-09-05T01:00:00Z',
  attachments: [],
};

function renderWorkflow() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } },
  });
  // Unrelated campaign resources are already loaded; exercise the report/template HTTP contract.
  for (const key of ['groups', 'pages', 'smtps']) queryClient.setQueryData([key], []);
  render(
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
        <ReportedMessageList />
        <TemplateList />
        <CampaignCreate />
      </QueryClientProvider>
    </MemoryRouter>,
  );
  return queryClient;
}

test('approval refreshes raw template responses in the library and campaign selector', async () => {
  let approved = false;
  let templateReads = 0;
  const requests: string[] = [];
  // Fixtures follow controllers/api/template.go and reported_message.go: no data envelope.
  const adapter: AxiosAdapter = async (config) => {
    requests.push(`${config.method} ${config.url}`);
    let data: unknown;
    if (config.url === '/templates/') {
      templateReads++;
      data = approved ? [template] : [];
    } else if (config.url === '/reported-messages/' && config.method === 'get') {
      const msg = { ...report, status: approved ? 'approved' : 'pending', converted_template_id: approved ? 42 : 0 };
      data = { data: [msg], total: 1, page: 1, per_page: 20 };
    } else if (config.url === '/reported-messages/1/approve' && config.method === 'post') {
      expect(JSON.parse(config.data)).toEqual({ name: template.name });
      approved = true;
      data = template;
    } else {
      throw new Error(`Unexpected request: ${config.method} ${config.url}`);
    }
    return { data, status: 200, statusText: 'OK', headers: {}, config };
  };
  client.defaults.adapter = adapter;
  const queryClient = renderWorkflow();

  await screen.findByText('No templates yet. Create your first email template.');
  fireEvent.click(await screen.findByRole('button', { name: 'Approve' }));
  fireEvent.change(screen.getByLabelText('Template name'), { target: { value: template.name } });
  fireEvent.click(screen.getAllByRole('button', { name: 'Approve' })[1]);

  expect(await screen.findByRole('option', { name: template.name })).toHaveValue('42');
  expect(screen.getAllByText(template.name)).toHaveLength(2);
  await waitFor(() => expect(screen.queryByText('Approve Reported Message')).not.toBeInTheDocument());
  expect(templateReads).toBe(2);
  expect(requests.filter((request) => request === 'post /reported-messages/1/approve')).toHaveLength(1);
  expect(queryClient.getQueryData(['templates'])).toEqual([template]);
});

test('a failed approval stays visible and does not create a template', async () => {
  client.defaults.adapter = async (config) => {
    if (config.method === 'post') throw new AxiosError('Conflict', 'ERR_BAD_REQUEST', config);
    return {
      data: config.url === '/templates/' ? [] : { data: [report], total: 1, page: 1, per_page: 20 },
      status: 200, statusText: 'OK', headers: {}, config,
    };
  };
  const queryClient = renderWorkflow();
  fireEvent.click(await screen.findByRole('button', { name: 'Approve' }));
  fireEvent.click(screen.getAllByRole('button', { name: 'Approve' })[1]);

  expect(await screen.findByRole('alert')).toHaveTextContent('Could not approve this report');
  expect(screen.getByText('Approve Reported Message')).toBeInTheDocument();
  expect(queryClient.getQueryData(['templates'])).toEqual([]);
});

test('template updates send the required id and return the raw object', async () => {
  client.defaults.adapter = async (config) => {
    expect(config.url).toBe('/templates/42');
    expect(config.method).toBe('put');
    expect(JSON.parse(config.data)).toEqual({ id: 42, name: template.name });
    return { data: template, status: 200, statusText: 'OK', headers: {}, config };
  };
  expect((await templatesApi.update(42, { name: template.name })).data).toEqual(template);
});

test('failed queries display errors instead of an empty report queue or template library', async () => {
  client.defaults.adapter = async (config) => {
    throw new AxiosError('Unavailable', 'ERR_NETWORK', config);
  };
  renderWorkflow();

  await waitFor(() => expect(screen.getAllByRole('alert')).toHaveLength(2));
  expect(screen.getByText(/Could not load reported messages/)).toBeInTheDocument();
  expect(screen.getByText(/Could not load templates/)).toBeInTheDocument();
  expect(screen.queryByText('No reported messages yet.')).not.toBeInTheDocument();
});

test('failed rejection leaves the pending report available with an error', async () => {
  client.defaults.adapter = async (config) => {
    if (config.method === 'post') throw new AxiosError('Conflict', 'ERR_BAD_REQUEST', config);
    return {
      data: config.url === '/templates/' ? [] : { data: [report], total: 1, page: 1, per_page: 20 },
      status: 200, statusText: 'OK', headers: {}, config,
    };
  };
  renderWorkflow();
  fireEvent.click(await screen.findByRole('button', { name: 'Reject' }));

  expect(await screen.findByRole('alert')).toHaveTextContent('Could not reject this report');
  expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument();
});
