import client from './client';
import type { ApiResponse, Template } from '../types';

interface CreateTemplateRequest {
  name: string;
  subject: string;
  text?: string;
  html?: string;
  envelope_sender?: string;
}

export const templatesApi = {
  getAll: () =>
    client.get<ApiResponse<Template[]>>('/templates/'),

  getById: (id: number) =>
    client.get<ApiResponse<Template>>(`/templates/${id}`),

  create: (data: CreateTemplateRequest) =>
    client.post<ApiResponse<Template>>('/templates/', data),

  update: (id: number, data: Partial<CreateTemplateRequest>) =>
    client.put<ApiResponse<Template>>(`/templates/${id}`, data),

  delete: (id: number) =>
    client.delete<ApiResponse<null>>(`/templates/${id}`),
};
