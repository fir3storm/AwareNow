import { useQuery, useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { campaignsApi } from '../../api/campaigns';
import { templatesApi } from '../../api/templates';
import { groupsApi } from '../../api/groups';
import { pagesApi } from '../../api/pages';
import { smtpApi } from '../../api/smtp';
import type { Template, Group, Page, SMTP } from '../../types';

export function CampaignCreate() {
  const navigate = useNavigate();
  const [formData, setFormData] = useState({
    name: '',
    template_id: 0,
    page_id: 0,
    smtp_id: 0,
    group_ids: [] as number[],
    url: '',
  });

  const { data: templates } = useQuery({
    queryKey: ['templates'],
    queryFn: async () => {
      const res = await templatesApi.getAll();
      return res.data;
    },
  });

  const { data: groups } = useQuery({
    queryKey: ['groups'],
    queryFn: async () => {
      const res = await groupsApi.getAll();
      return res.data.data;
    },
  });

  const { data: pages } = useQuery({
    queryKey: ['pages'],
    queryFn: async () => {
      const res = await pagesApi.getAll();
      return res.data.data;
    },
  });

  const { data: smtps } = useQuery({
    queryKey: ['smtps'],
    queryFn: async () => {
      const res = await smtpApi.getAll();
      return res.data.data;
    },
  });

  const createMutation = useMutation({
    mutationFn: (data: typeof formData) => campaignsApi.create(data),
    onSuccess: () => {
      navigate('/campaigns');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createMutation.mutate(formData);
  };

  const handleGroupToggle = (groupId: number) => {
    setFormData((prev) => ({
      ...prev,
      group_ids: prev.group_ids.includes(groupId)
        ? prev.group_ids.filter((id) => id !== groupId)
        : [...prev.group_ids, groupId],
    }));
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Create Campaign</h1>
        <p className="text-gray-500 mt-1">Set up a new phishing simulation</p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <Card>
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Campaign Details</h3>
          <div className="space-y-4">
            <div>
              <label className="label">Campaign Name</label>
              <input
                type="text"
                className="input"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder="Q4 Security Awareness Training"
                required
              />
            </div>
            <div>
              <label className="label">Campaign URL</label>
              <input
                type="url"
                className="input"
                value={formData.url}
                onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                placeholder="https://training.example.com"
                required
              />
              <p className="text-sm text-gray-500 mt-1">The URL targets will see in emails</p>
            </div>
          </div>
        </Card>

        <Card>
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Email Template</h3>
          <select
            className="input"
            value={formData.template_id}
            onChange={(e) => setFormData({ ...formData, template_id: Number(e.target.value) })}
            required
          >
            <option value={0}>Select a template</option>
            {templates?.map((template: Template) => (
              <option key={template.id} value={template.id}>
                {template.name}
              </option>
            ))}
          </select>
        </Card>

        <Card>
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Landing Page</h3>
          <select
            className="input"
            value={formData.page_id}
            onChange={(e) => setFormData({ ...formData, page_id: Number(e.target.value) })}
            required
          >
            <option value={0}>Select a landing page</option>
            {pages?.map((page: Page) => (
              <option key={page.id} value={page.id}>
                {page.name}
              </option>
            ))}
          </select>
        </Card>

        <Card>
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Sending Profile</h3>
          <select
            className="input"
            value={formData.smtp_id}
            onChange={(e) => setFormData({ ...formData, smtp_id: Number(e.target.value) })}
            required
          >
            <option value={0}>Select a sending profile</option>
            {smtps?.map((smtp: SMTP) => (
              <option key={smtp.id} value={smtp.id}>
                {smtp.name} ({smtp.host})
              </option>
            ))}
          </select>
        </Card>

        <Card>
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Target Groups</h3>
          <div className="space-y-2">
            {groups?.map((group: Group) => (
              <label
                key={group.id}
                className="flex items-center gap-3 p-3 bg-gray-50 rounded-lg cursor-pointer hover:bg-gray-100 transition-colors"
              >
                <input
                  type="checkbox"
                  checked={formData.group_ids.includes(group.id)}
                  onChange={() => handleGroupToggle(group.id)}
                  className="w-4 h-4 text-indigo-600 rounded border-gray-300 focus:ring-indigo-500"
                />
                <div>
                  <p className="font-medium text-gray-900">{group.name}</p>
                  <p className="text-sm text-gray-500">{group.targets?.length ?? 0} targets</p>
                </div>
              </label>
            ))}
          </div>
        </Card>

        <div className="flex justify-end gap-3">
          <Button type="button" variant="secondary" onClick={() => navigate('/campaigns')}>
            Cancel
          </Button>
          <Button type="submit" loading={createMutation.isPending}>
            Launch Campaign
          </Button>
        </div>
      </form>
    </div>
  );
}
