import client from './client';
import type { ApiResponse, Campaign, CampaignResult } from '../types';

interface CreateCampaignRequest {
  name: string;
  template_id: number;
  page_id: number;
  smtp_id: number;
  group_ids: number[];
  url: string;
  launch_date?: string;
  send_by_date?: string;
}

export const campaignsApi = {
  getAll: (params?: { page?: number; per_page?: number }) =>
    client.get<ApiResponse<Campaign[]>>('/campaigns/', { params }),

  getById: (id: number) =>
    client.get<ApiResponse<Campaign>>(`/campaigns/${id}`),

  getSummary: () =>
    client.get<{ total: number; campaigns: Campaign[] }>('/campaigns/summary'),

  getResults: (id: number) =>
    client.get<ApiResponse<CampaignResult[]>>(`/campaigns/${id}/results`),

  create: (data: CreateCampaignRequest) =>
    client.post<ApiResponse<Campaign>>('/campaigns/', data),

  complete: (id: number) =>
    client.get<ApiResponse<Campaign>>(`/campaigns/${id}/complete`),

  delete: (id: number) =>
    client.delete<ApiResponse<null>>(`/campaigns/${id}`),
};
