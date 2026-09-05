import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Mail,
  Users,
  FileText,
  Globe,
  Send,
  Settings,
  Shield,
  Flag,
} from 'lucide-react';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/campaigns', icon: Mail, label: 'Campaigns' },
  { to: '/templates', icon: FileText, label: 'Email Templates' },
  { to: '/reported-messages', icon: Flag, label: 'Reported Messages' },
  { to: '/groups', icon: Users, label: 'Users & Groups' },
  { to: '/pages', icon: Globe, label: 'Landing Pages' },
  { to: '/sending-profiles', icon: Send, label: 'Sending Profiles' },
  { to: '/settings', icon: Settings, label: 'Settings' },
];

export function Sidebar() {
  return (
    <aside className="w-64 bg-white border-r border-gray-200 min-h-screen flex flex-col">
      <div className="p-6 border-b border-gray-200">
        <div className="flex items-center gap-3">
          <Shield className="w-8 h-8 text-indigo-600" />
          <span className="text-xl font-bold text-gray-900">AwareNow</span>
        </div>
      </div>

      <nav className="flex-1 p-4 space-y-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              `sidebar-link ${isActive ? 'active' : ''}`
            }
          >
            <item.icon className="w-5 h-5" />
            <span>{item.label}</span>
          </NavLink>
        ))}
      </nav>

      <div className="p-4 border-t border-gray-200">
        <p className="text-xs text-gray-500 text-center">
          Security Awareness Platform
        </p>
      </div>
    </aside>
  );
}
