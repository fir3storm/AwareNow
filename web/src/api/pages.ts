import client from './client';
import type { ApiResponse, Page } from '../types';

interface CreatePageRequest {
  name: string;
  html: string;
  capture_credentials?: boolean;
  capture_passwords?: boolean;
  redirect_url?: string;
}

export const pagesApi = {
  getAll: () =>
    client.get<ApiResponse<Page[]>>('/pages/'),

  getById: (id: number) =>
    client.get<ApiResponse<Page>>(`/pages/${id}`),

  create: (data: CreatePageRequest) =>
    client.post<ApiResponse<Page>>('/pages/', data),

  update: (id: number, data: Partial<CreatePageRequest>) =>
    client.put<ApiResponse<Page>>(`/pages/${id}`, data),

  delete: (id: number) =>
    client.delete<ApiResponse<null>>(`/pages/${id}`),

  importSite: (url: string) =>
    client.post<ApiResponse<Page>>('/import/site', { url }),
};
