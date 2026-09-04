import client from './client';
import type { ApiResponse, SMTP } from '../types';

interface CreateSMTPRequest {
  name: string;
  host: string;
  username?: string;
  password?: string;
  from_address: string;
  ignore_cert_errors?: boolean;
  interface?: string;
}

export const smtpApi = {
  getAll: () =>
    client.get<ApiResponse<SMTP[]>>('/smtp/'),

  getById: (id: number) =>
    client.get<ApiResponse<SMTP>>(`/smtp/${id}`),

  create: (data: CreateSMTPRequest) =>
    client.post<ApiResponse<SMTP>>('/smtp/', data),

  update: (id: number, data: Partial<CreateSMTPRequest>) =>
    client.put<ApiResponse<SMTP>>(`/smtp/${id}`, data),

  delete: (id: number) =>
    client.delete<ApiResponse<null>>(`/smtp/${id}`),

  sendTestEmail: (smtpId: number, email: string) =>
    client.post<ApiResponse<null>>('/util/send_test_email', {
      smtp_id: smtpId,
      email: email,
    }),
};
