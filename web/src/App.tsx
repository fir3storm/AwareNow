import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Layout } from './components/layout/Layout';
import { Login } from './pages/Login';
import { Dashboard } from './pages/Dashboard';
import { CampaignList } from './pages/Campaigns/CampaignList';
import { CampaignCreate } from './pages/Campaigns/CampaignCreate';
import { CampaignResults } from './pages/Campaigns/CampaignResults';
import { TemplateList } from './pages/Templates/TemplateList';
import { GroupList } from './pages/Groups/GroupList';
import { PageList } from './pages/Pages/PageList';
import { SMTPList } from './pages/SendingProfiles/SMTPList';
import { Settings } from './pages/Settings/Settings';
import { useAuthStore } from './store/authStore';
import { AwarenessOverview } from './control-plane/AwarenessOverview';
import { ProvisioningStatus } from './control-plane/ProvisioningStatus';
import { TenantContextBanner } from './control-plane/TenantContextBanner';
import type { AwarenessMetrics, TenantView } from './control-plane/types';
import './App.css';

const developmentTenantFixture: TenantView = {
  id: 'development-fixture-only',
  displayName: 'Development workspace',
  slug: 'development-fixture',
  lifecycle: 'PROVISIONING',
};

const developmentAwarenessMetrics: AwarenessMetrics = {
  sent: 0,
  opened: 0,
  clicked: 0,
  reported: 0,
  trainingCompleted: 0,
};

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore();
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />;
}

function PublicRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore();
  return isAuthenticated ? <Navigate to="/" replace /> : <>{children}</>;
}

function DashboardWithDevelopmentTenantFixture() {
  return (
    <div className="space-y-6">
      <aside className="control-plane-development-fixture" aria-label="Development fixture data">
        <strong>Development fixture</strong>
        <span>Sample aggregate-only values. No live tenant, recipient, or campaign data is shown.</span>
      </aside>
      <div className="control-plane-dashboard-summary">
        <TenantContextBanner tenant={developmentTenantFixture} />
        <ProvisioningStatus tenant={developmentTenantFixture} />
      </div>
      <AwarenessOverview metrics={developmentAwarenessMetrics} />
      <Dashboard />
    </div>
  );
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<PublicRoute><Login /></PublicRoute>} />
      <Route
        path="/"
        element={
          <PrivateRoute>
            <Layout />
          </PrivateRoute>
        }
      >
        <Route index element={<DashboardWithDevelopmentTenantFixture />} />
        <Route path="campaigns" element={<CampaignList />} />
        <Route path="campaigns/new" element={<CampaignCreate />} />
        <Route path="campaigns/:id" element={<CampaignResults />} />
        <Route path="templates" element={<TemplateList />} />
        <Route path="groups" element={<GroupList />} />
        <Route path="pages" element={<PageList />} />
        <Route path="sending-profiles" element={<SMTPList />} />
        <Route path="settings" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
