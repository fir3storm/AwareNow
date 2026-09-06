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
    client.get<Template[]>('/templates/'),

  getById: (id: number) =>
    client.get<Template>(`/templates/${id}`),

  create: (data: CreateTemplateRequest) =>
    client.post<Template>('/templates/', data),

  update: (id: number, data: Partial<CreateTemplateRequest>) =>
    client.put<Template>(`/templates/${id}`, { ...data, id }),

  delete: (id: number) =>
    client.delete<ApiResponse<null>>(`/templates/${id}`),
};
