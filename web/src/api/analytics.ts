import client from './client';
import type { ApiResponse, AnalyticsOverview, TimelineData, DepartmentStats } from '../types';

export const analyticsApi = {
  getOverview: () =>
    client.get<ApiResponse<AnalyticsOverview>>('/analytics/overview'),

  getTimeline: (campaignId?: number) =>
    client.get<ApiResponse<TimelineData[]>>(
      campaignId ? `/analytics/campaigns/${campaignId}/timeline` : '/analytics/timeline'
    ),

  getDepartments: () =>
    client.get<ApiResponse<DepartmentStats[]>>('/analytics/departments'),

  getTrends: (period: string = '30d') =>
    client.get<ApiResponse<TimelineData[]>>('/analytics/trends', { params: { period } }),

  getRiskScore: () =>
    client.get<ApiResponse<{ score: number }>>('/analytics/risk-score'),

  export: (format: 'csv' | 'pdf' | 'xlsx', campaignId?: number) =>
    client.get<Blob>('/analytics/export', {
      params: { format, campaign_id: campaignId },
      responseType: 'blob',
    }),
};
