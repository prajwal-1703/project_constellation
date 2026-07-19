import { useConstellation, formatBytes } from '../hooks/useConstellation';
import { Activity, Cpu, HardDrive, Server } from 'lucide-react';
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  BarChart, Bar
} from 'recharts';

// Mock timeseries data for the MVP demo (since we don't have historical metrics yet)
const mockTimeseries = Array.from({ length: 20 }, (_, i) => ({
  time: `${i}:00`,
  cpu: Math.floor(Math.random() * 60) + 20,
  mem: Math.floor(Math.random() * 40) + 40,
}));

const StatCard = ({ title, value, subtitle, icon: Icon, color }: any) => (
  <div className="glass-panel" style={{ padding: '24px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <span style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', fontWeight: 500 }}>{title}</span>
      <div style={{ 
        background: `rgba(${color}, 0.1)`, 
        color: `rgb(${color})`,
        padding: '8px', 
        borderRadius: '8px'
      }}>
        <Icon size={20} />
      </div>
    </div>
    <div>
      <div style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--text-primary)', marginBottom: '4px' }}>
        {value}
      </div>
      <div style={{ fontSize: '0.85rem', color: 'var(--text-tertiary)' }}>
        {subtitle}
      </div>
    </div>
  </div>
);

const Dashboard = () => {
  const { status, connected } = useConstellation(localStorage.getItem('constellation_token'));

  if (!status) {
    return <div className="animate-pulse">Loading cluster status...</div>;
  }

  const { pool, task_stats, cluster } = status;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '32px' }}>
      
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h1 style={{ fontSize: '2rem', marginBottom: '8px' }}>Cluster Overview</h1>
          <p style={{ color: 'var(--text-secondary)' }}>Monitoring cluster <strong style={{ color: 'var(--text-primary)' }}>{cluster?.name || 'Constellation'}</strong></p>
        </div>
        <div className={`status-badge ${connected ? 'status-online' : 'status-offline'}`}>
          {connected ? 'Live Updates Active' : 'Disconnected'}
        </div>
      </div>

      {/* Primary Stats */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '24px' }}>
        <StatCard 
          title="Active Nodes" 
          value={`${pool?.online_nodes || 0} / ${pool?.total_nodes || 0}`}
          subtitle="Machines in compute pool"
          icon={Server}
          color="59, 130, 246" // blue
        />
        <StatCard 
          title="Running Tasks" 
          value={task_stats?.running || 0}
          subtitle={`${task_stats?.queued || 0} queued tasks`}
          icon={Activity}
          color="16, 185, 129" // green
        />
        <StatCard 
          title="Compute Cores" 
          value={pool?.total_cpu_cores || 0}
          subtitle={`${pool?.available_cpu_cores || 0} available`}
          icon={Cpu}
          color="139, 92, 246" // purple
        />
        <StatCard 
          title="Total Memory" 
          value={formatBytes(pool?.total_memory_bytes || 0)}
          subtitle={`${formatBytes(pool?.available_memory_bytes || 0)} available`}
          icon={HardDrive}
          color="245, 158, 11" // orange
        />
      </div>

      {/* Charts Section */}
      <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '24px' }}>
        
        <div className="glass-panel" style={{ padding: '24px' }}>
          <h3 style={{ marginBottom: '24px', color: 'var(--text-secondary)', fontSize: '1rem' }}>Cluster Utilization</h3>
          <div style={{ height: '300px' }}>
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={mockTimeseries}>
                <defs>
                  <linearGradient id="colorCpu" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="var(--primary)" stopOpacity={0.3}/>
                    <stop offset="95%" stopColor="var(--primary)" stopOpacity={0}/>
                  </linearGradient>
                  <linearGradient id="colorMem" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="var(--accent)" stopOpacity={0.3}/>
                    <stop offset="95%" stopColor="var(--accent)" stopOpacity={0}/>
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.05)" vertical={false} />
                <XAxis dataKey="time" stroke="var(--text-tertiary)" tick={{fill: 'var(--text-tertiary)'}} axisLine={false} tickLine={false} />
                <YAxis stroke="var(--text-tertiary)" tick={{fill: 'var(--text-tertiary)'}} axisLine={false} tickLine={false} />
                <Tooltip 
                  contentStyle={{ backgroundColor: 'var(--bg-panel)', borderColor: 'var(--border-color)', borderRadius: 'var(--radius-sm)' }}
                  itemStyle={{ color: 'var(--text-primary)' }}
                />
                <Area type="monotone" dataKey="cpu" name="CPU (%)" stroke="var(--primary)" fillOpacity={1} fill="url(#colorCpu)" />
                <Area type="monotone" dataKey="mem" name="Memory (%)" stroke="var(--accent)" fillOpacity={1} fill="url(#colorMem)" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="glass-panel" style={{ padding: '24px' }}>
          <h3 style={{ marginBottom: '24px', color: 'var(--text-secondary)', fontSize: '1rem' }}>Task Distribution</h3>
          <div style={{ height: '300px' }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={[
                { name: 'Running', val: task_stats?.running || 0, fill: 'var(--primary)' },
                { name: 'Queued', val: task_stats?.queued || 0, fill: 'var(--warning)' },
                { name: 'Done', val: task_stats?.completed || 0, fill: 'var(--success)' },
                { name: 'Failed', val: task_stats?.failed || 0, fill: 'var(--danger)' },
              ]}>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.05)" vertical={false} />
                <XAxis dataKey="name" stroke="var(--text-tertiary)" axisLine={false} tickLine={false} />
                <YAxis stroke="var(--text-tertiary)" axisLine={false} tickLine={false} allowDecimals={false} />
                <Tooltip 
                  cursor={{fill: 'var(--bg-panel-hover)'}}
                  contentStyle={{ backgroundColor: 'var(--bg-panel)', borderColor: 'var(--border-color)', borderRadius: 'var(--radius-sm)' }}
                />
                <Bar dataKey="val" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
