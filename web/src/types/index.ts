export interface User {
  id: number;
  username: string;
  api_key: string;
  role: {
    id: number;
    name: string;
    permissions: { slug: string }[];
  };
  last_login: string;
  password_change_required: boolean;
}

export interface Campaign {
  id: number;
  name: string;
  status: 'Created' | 'Queued' | 'In Progress' | 'Emails Sent' | 'Completed';
  created_date: string;
  launch_date: string;
  send_by_date: string | null;
  completed_date: string | null;
  url: string;
  template: Template | null;
  page: Page | null;
  smtp: SMTP | null;
  results: CampaignResult[];
  stats: {
    total: number;
    sent: number;
    opened: number;
    clicked: number;
    submitted_data: number;
    reported: number;
    errors: number;
  };
}

export interface CampaignResult {
  id: number;
  rid: string;
  email: string;
  first_name: string;
  last_name: string;
  position: string;
  status: 'Scheduled' | 'Sending' | 'Email Sent' | 'Email Opened' | 'Clicked Link' | 'Submitted Data' | 'Reported' | 'Error';
  ip: string;
  latitude: number;
  longitude: number;
  send_date: string;
  reported: boolean;
}

export interface Template {
  id: number;
  name: string;
  envelope_sender: string;
  subject: string;
  text: string;
  html: string;
  modified_date: string;
  attachments: Attachment[];
}

export interface Attachment {
  id: number;
  content: string;
  type: string;
  name: string;
}

export interface Group {
  id: number;
  name: string;
  modified_date: string;
  targets: Target[];
}

export interface Target {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
  position: string;
}

export interface Page {
  id: number;
  name: string;
  html: string;
  capture_credentials: boolean;
  capture_passwords: boolean;
  redirect_url: string;
  modified_date: string;
}

export interface SMTP {
  id: number;
  name: string;
  interface: string;
  host: string;
  username: string;
  password: string;
  from_address: string;
  ignore_cert_errors: boolean;
  modified_date: string;
  headers: SMTPHeader[];
}

export interface SMTPHeader {
  key: string;
  value: string;
}

export interface AnalyticsOverview {
  total_campaigns: number;
  emails_sent: number;
  open_rate: number;
  click_rate: number;
  submit_rate: number;
  report_rate: number;
  avg_time_to_click: string;
  risk_score: number;
}

export interface TimelineData {
  date: string;
  opens: number;
  clicks: number;
  submits: number;
}

export interface DepartmentStats {
  department: string;
  users_count: number;
  click_rate: number;
  submit_rate: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

export interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}
