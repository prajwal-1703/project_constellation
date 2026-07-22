import { useEffect, useState } from 'react';
import { PieChart, Pie, Cell, AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

interface AnalyticsData {
  tasks_summary: {
    success: number;
    failed: number;
    running: number;
    total: number;
  };
  utilization_history: Array<{
    date: string;
    cpu_usage: number;
    memory_usage: number;
  }>;
  cost_savings: string;
  active_nodes: number;
}

const COLORS = ['#10b981', '#ef4444', '#3b82f6'];

const Analytics = () => {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const token = localStorage.getItem('constellation_token');

  useEffect(() => {
    fetch('/api/v1/analytics', {
      headers: { 'Authorization': `Bearer ${token}` }
    })
      .then(res => res.json())
      .then(d => setData(d))
      .catch(console.error);
  }, [token]);

  if (!data) return <div style={{ padding: '24px' }}>Loading analytics...</div>;

  const pieData = [
    { name: 'Completed', value: data.tasks_summary.success },
    { name: 'Failed', value: data.tasks_summary.failed },
    { name: 'Running', value: data.tasks_summary.running },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px', padding: '24px' }}>
      <div>
        <h1 style={{ fontSize: '2rem', marginBottom: '8px' }}>Analytics & Reporting</h1>
        <p style={{ color: 'var(--text-secondary)' }}>Enterprise insights on cluster utilization and cost savings.</p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '16px' }}>
        <div className="glass-panel" style={{ padding: '20px' }}>
          <h3 style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', marginBottom: '8px' }}>Total Tasks</h3>
          <p style={{ fontSize: '1.8rem', fontWeight: 600 }}>{data.tasks_summary.total}</p>
        </div>
        <div className="glass-panel" style={{ padding: '20px' }}>
          <h3 style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', marginBottom: '8px' }}>Success Rate</h3>
          <p style={{ fontSize: '1.8rem', fontWeight: 600, color: 'var(--success)' }}>
            {data.tasks_summary.total ? Math.round((data.tasks_summary.success / data.tasks_summary.total) * 100) : 0}%
          </p>
        </div>
        <div className="glass-panel" style={{ padding: '20px' }}>
          <h3 style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', marginBottom: '8px' }}>Active Nodes</h3>
          <p style={{ fontSize: '1.8rem', fontWeight: 600, color: 'var(--primary)' }}>{data.active_nodes}</p>
        </div>
        <div className="glass-panel" style={{ padding: '20px' }}>
          <h3 style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', marginBottom: '8px' }}>Est. Cost Savings</h3>
          <p style={{ fontSize: '1.8rem', fontWeight: 600, color: 'var(--warning)' }}>{data.cost_savings}</p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '24px' }}>
        <div className="glass-panel" style={{ padding: '24px', height: '400px' }}>
          <h2 style={{ fontSize: '1.2rem', marginBottom: '16px' }}>Cluster Utilization (7 Days)</h2>
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data.utilization_history}>
              <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.1)" />
              <XAxis dataKey="date" stroke="rgba(255,255,255,0.5)" />
              <YAxis stroke="rgba(255,255,255,0.5)" />
              <Tooltip contentStyle={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)' }} />
              <Area type="monotone" dataKey="cpu_usage" stackId="1" stroke="#3b82f6" fill="#3b82f6" fillOpacity={0.6} name="CPU (%)" />
              <Area type="monotone" dataKey="memory_usage" stackId="2" stroke="#8b5cf6" fill="#8b5cf6" fillOpacity={0.6} name="Memory (%)" />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        <div className="glass-panel" style={{ padding: '24px', height: '400px', display: 'flex', flexDirection: 'column' }}>
          <h2 style={{ fontSize: '1.2rem', marginBottom: '16px' }}>Task Distribution</h2>
          <div style={{ flex: 1, position: 'relative' }}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={pieData} cx="50%" cy="50%" innerRadius={60} outerRadius={100} paddingAngle={5} dataKey="value">
                  {pieData.map((_entry, index) => (
                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip contentStyle={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)' }} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Analytics;
