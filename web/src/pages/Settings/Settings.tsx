import { useState } from 'react';
import { Key, Lock, User } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { useAuthStore } from '../../store/authStore';
import { authApi } from '../../api/auth';

export function Settings() {
  const { user } = useAuthStore();
  const [passwordForm, setPasswordForm] = useState({
    current_password: '',
    new_password: '',
    confirm_password: '',
  });
  const [passwordError, setPasswordError] = useState('');
  const [passwordSuccess, setPasswordSuccess] = useState('');
  const [loading, setLoading] = useState(false);

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess('');

    if (passwordForm.new_password !== passwordForm.confirm_password) {
      setPasswordError('New passwords do not match');
      return;
    }

    if (passwordForm.new_password.length < 8) {
      setPasswordError('Password must be at least 8 characters');
      return;
    }

    setLoading(true);
    try {
      await authApi.changePassword({
        current_password: passwordForm.current_password,
        new_password: passwordForm.new_password,
      });
      setPasswordSuccess('Password changed successfully');
      setPasswordForm({ current_password: '', new_password: '', confirm_password: '' });
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } };
      setPasswordError(error.response?.data?.message || 'Failed to change password');
    } finally {
      setLoading(false);
    }
  };

  const handleResetApiKey = async () => {
    if (confirm('Are you sure you want to reset your API key? This will invalidate the current key.')) {
      try {
        await authApi.resetApiKey();
        alert('API key reset successfully');
      } catch {
        alert('Failed to reset API key');
      }
    }
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
        <p className="text-gray-500 mt-1">Manage your account settings</p>
      </div>

      <Card>
        <div className="flex items-center gap-4 mb-6">
          <div className="p-3 bg-indigo-50 rounded-full">
            <User className="w-6 h-6 text-indigo-600" />
          </div>
          <div>
            <h3 className="text-lg font-semibold text-gray-900">Account Information</h3>
            <p className="text-sm text-gray-500">Your account details</p>
          </div>
        </div>
        <div className="space-y-4">
          <div>
            <label className="label">Username</label>
            <input type="text" className="input bg-gray-50" value={user?.username ?? ''} disabled />
          </div>
          <div>
            <label className="label">Role</label>
            <input
              type="text"
              className="input bg-gray-50 capitalize"
              value={user?.role?.name ?? ''}
              disabled
            />
          </div>
        </div>
      </Card>

      <Card>
        <div className="flex items-center gap-4 mb-6">
          <div className="p-3 bg-yellow-50 rounded-full">
            <Lock className="w-6 h-6 text-yellow-600" />
          </div>
          <div>
            <h3 className="text-lg font-semibold text-gray-900">Change Password</h3>
            <p className="text-sm text-gray-500">Update your password</p>
          </div>
        </div>

        {passwordError && (
          <div className="p-3 bg-red-50 text-red-700 rounded-lg mb-4 text-sm">
            {passwordError}
          </div>
        )}
        {passwordSuccess && (
          <div className="p-3 bg-green-50 text-green-700 rounded-lg mb-4 text-sm">
            {passwordSuccess}
          </div>
        )}

        <form onSubmit={handlePasswordChange} className="space-y-4">
          <div>
            <label className="label">Current Password</label>
            <input
              type="password"
              className="input"
              value={passwordForm.current_password}
              onChange={(e) => setPasswordForm({ ...passwordForm, current_password: e.target.value })}
              required
            />
          </div>
          <div>
            <label className="label">New Password</label>
            <input
              type="password"
              className="input"
              value={passwordForm.new_password}
              onChange={(e) => setPasswordForm({ ...passwordForm, new_password: e.target.value })}
              required
              minLength={8}
            />
          </div>
          <div>
            <label className="label">Confirm New Password</label>
            <input
              type="password"
              className="input"
              value={passwordForm.confirm_password}
              onChange={(e) => setPasswordForm({ ...passwordForm, confirm_password: e.target.value })}
              required
              minLength={8}
            />
          </div>
          <Button type="submit" loading={loading}>
            Update Password
          </Button>
        </form>
      </Card>

      <Card>
        <div className="flex items-center gap-4 mb-6">
          <div className="p-3 bg-red-50 rounded-full">
            <Key className="w-6 h-6 text-red-600" />
          </div>
          <div>
            <h3 className="text-lg font-semibold text-gray-900">API Key</h3>
            <p className="text-sm text-gray-500">Manage your API key</p>
          </div>
        </div>
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-gray-500 mb-1">Current API Key</p>
            <code className="text-sm bg-gray-100 px-3 py-1 rounded">
              {user?.api_key?.substring(0, 8)}...{user?.api_key?.substring(user.api_key.length - 8)}
            </code>
          </div>
          <Button variant="danger" onClick={handleResetApiKey}>
            Reset API Key
          </Button>
        </div>
      </Card>
    </div>
  );
}
