import client from './client';
import type { Template } from '../types';

export type ReportedMessageStatus = 'pending' | 'approved' | 'rejected';

export interface ReportedMessage {
  id: number;
  reporter_email: string;
  subject: string;
  body_text: string;
  body_html: string;
  status: ReportedMessageStatus;
  converted_template_id: number;
  reviewed_by: string;
  created_at: string;
  reviewed_at: string;
}

export const reportedMessagesApi = {
  getAll: (status?: ReportedMessageStatus) =>
    client.get<ReportedMessage[]>('/reported-messages/', { params: status ? { status } : undefined }),

  getById: (id: number) =>
    client.get<ReportedMessage>(`/reported-messages/${id}`),

  approve: (id: number, name: string) =>
    client.post<Template>(`/reported-messages/${id}/approve`, { name }),

  reject: (id: number) =>
    client.post<{ success: boolean; message: string }>(`/reported-messages/${id}/reject`),
};
