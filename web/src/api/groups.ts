import client from './client';
import type { ApiResponse, Group, Target } from '../types';

interface CreateGroupRequest {
  name: string;
  targets: Omit<Target, 'id'>[];
}

export const groupsApi = {
  getAll: () =>
    client.get<ApiResponse<Group[]>>('/groups/'),

  getById: (id: number) =>
    client.get<ApiResponse<Group>>(`/groups/${id}`),

  getSummary: () =>
    client.get<ApiResponse<{ groups: Group[] }>>('/groups/summary'),

  create: (data: CreateGroupRequest) =>
    client.post<ApiResponse<Group>>('/groups/', data),

  update: (id: number, data: Partial<CreateGroupRequest>) =>
    client.put<ApiResponse<Group>>(`/groups/${id}`, data),

  delete: (id: number) =>
    client.delete<ApiResponse<null>>(`/groups/${id}`),

  importCSV: (csvData: string) =>
    client.post<ApiResponse<Group>>('/import/group', { csv: csvData }),
};
