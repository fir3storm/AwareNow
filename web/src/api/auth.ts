import client from './client';
import type { ApiResponse, User } from '../types';

interface LoginRequest {
  username: string;
  password: string;
}

interface LoginResponse {
  user: User;
  token: string;
}

export const authApi = {
  login: (data: LoginRequest) =>
    client.post<ApiResponse<LoginResponse>>('/login', data),

  logout: () =>
    client.post('/logout'),

  getProfile: () =>
    client.get<ApiResponse<User>>('/users/me'),

  changePassword: (data: { current_password: string; new_password: string }) =>
    client.post('/reset', data),

  resetApiKey: () =>
    client.post<ApiResponse<{ api_key: string }>>('/reset'),
};
