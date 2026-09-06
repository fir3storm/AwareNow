import client from './client';
import type { AnalyticsOverview, TimelineData, DepartmentStats } from '../types';

export const analyticsApi = {
  getOverview: () =>
    client.get<AnalyticsOverview>('/analytics/overview'),

  getTimeline: (campaignId?: number) =>
    client.get<TimelineData[]>(
      campaignId ? `/analytics/campaigns/${campaignId}/timeline` : '/analytics/timeline'
    ),

  getDepartments: () =>
    client.get<DepartmentStats[]>('/analytics/departments'),

  getTrends: (period: string = '30d') =>
    client.get<TimelineData[]>('/analytics/trends', { params: { period } }),

  getRiskScore: () =>
    client.get<{ score: number }>('/analytics/risk-score'),

  export: (format: 'csv' | 'pdf' | 'xlsx', campaignId?: number) =>
    client.get<Blob>('/analytics/export', {
      params: { format, campaign_id: campaignId },
      responseType: 'blob',
    }),
};
