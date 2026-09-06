import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { Card } from '../../components/ui/Card';
import { Table } from '../../components/ui/Table';
import { assessmentsApi } from '../../api/assessments';
import type { Assessment } from '../../api/assessments';

const statusStyles: Record<string, string> = {
  draft: 'bg-gray-100 text-gray-600',
  active: 'bg-green-100 text-green-800',
  completed: 'bg-indigo-100 text-indigo-800',
};

export function AssessmentList() {
  const navigate = useNavigate();

  const { data: assessments, isLoading, isError } = useQuery({
    queryKey: ['assessments'],
    queryFn: async () => {
      const res = await assessmentsApi.getAll();
      return res.data;
    },
  });

  const columns = [
    {
      key: 'name',
      header: 'Name',
      render: (assessment: Assessment) => (
        <span className="font-medium">{assessment.name}</span>
      ),
    },
    {
      key: 'skill_tag',
      header: 'Skill Tag',
      render: (assessment: Assessment) => (
        <span className="text-sm text-gray-700">{assessment.skill_tag}</span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (assessment: Assessment) => (
        <span
          className={`inline-block px-2 py-1 rounded-full text-xs font-medium capitalize ${
            statusStyles[assessment.status] || 'bg-gray-100 text-gray-600'
          }`}
        >
          {assessment.status}
        </span>
      ),
    },
    {
      key: 'created_at',
      header: 'Created',
      render: (assessment: Assessment) => (
        <span className="text-sm text-gray-500">
          {new Date(assessment.created_at).toLocaleDateString()}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Assessments</h1>
        <p className="text-gray-500 mt-1">
          Proof-of-resilience measurement exercises. Select one to view its evidence.
        </p>
      </div>

      <Card>
        {isError ? (
          <p role="alert" className="text-red-700">
            Could not load assessments. Refresh the page to try again.
          </p>
        ) : isLoading ? (
          <div className="animate-pulse space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-12 bg-gray-100 rounded"></div>
            ))}
          </div>
        ) : (
          <Table
            columns={columns}
            data={assessments ?? []}
            keyExtractor={(assessment) => assessment.id}
            onRowClick={(assessment) => navigate(`/assessments/${assessment.id}/evidence`)}
            emptyMessage="No assessments yet."
          />
        )}
      </Card>
    </div>
  );
}
