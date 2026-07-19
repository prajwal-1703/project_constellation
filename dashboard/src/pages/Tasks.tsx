import { useConstellation } from '../hooks/useConstellation';
import { Activity, Play, Square, RotateCcw, Terminal, TerminalSquare } from 'lucide-react';

const Tasks = () => {
  const { tasks } = useConstellation(localStorage.getItem('constellation_token'));

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'running': return <span className="status-badge status-running">Running</span>;
      case 'completed': return <span className="status-badge status-online">Completed</span>;
      case 'failed': return <span className="status-badge status-offline">Failed</span>;
      case 'cancelled': return <span className="status-badge" style={{ background: 'rgba(255,255,255,0.1)', color: 'var(--text-secondary)' }}>Cancelled</span>;
      default: return <span className="status-badge status-queued">{status}</span>;
    }
  };

  const getPriorityColor = (p: string) => {
    switch (p) {
      case 'critical': return 'var(--danger)';
      case 'high': return 'var(--warning)';
      case 'low': return 'var(--text-tertiary)';
      default: return 'var(--primary)';
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1 style={{ fontSize: '2rem', marginBottom: '8px' }}>Task Queue</h1>
          <p style={{ color: 'var(--text-secondary)' }}>Monitor and manage distributed workloads.</p>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <button className="btn btn-outline">Clear Completed</button>
          <button className="btn btn-primary" style={{ gap: '8px' }}>
            <Play size={16} fill="currentColor" /> Submit Task
          </button>
        </div>
      </div>

      <div className="glass-panel table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Task Name / ID</th>
              <th>Status</th>
              <th>Command</th>
              <th>Priority</th>
              <th>Node</th>
              <th>Exit Code</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {tasks.length === 0 ? (
              <tr>
                <td colSpan={7} style={{ textAlign: 'center', padding: '32px', color: 'var(--text-tertiary)' }}>
                  No tasks found in the system.
                </td>
              </tr>
            ) : (
              tasks.map(task => (
                <tr key={task.id}>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                      <div style={{ background: 'var(--bg-panel-hover)', padding: '8px', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                        <TerminalSquare size={18} color="var(--text-secondary)" />
                      </div>
                      <div>
                        <div style={{ fontWeight: 500, color: 'var(--text-primary)' }}>{task.name}</div>
                        <div style={{ fontSize: '0.75rem', color: 'var(--text-tertiary)' }}>{task.id}</div>
                      </div>
                    </div>
                  </td>
                  <td>{getStatusBadge(task.status)}</td>
                  <td>
                    <code style={{ 
                      background: 'rgba(0,0,0,0.3)', 
                      padding: '4px 8px', 
                      borderRadius: '4px',
                      fontSize: '0.8rem',
                      color: 'var(--text-secondary)',
                      whiteSpace: 'nowrap',
                      maxWidth: '200px',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      display: 'inline-block'
                    }}>
                      {task.command}
                    </code>
                  </td>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <div style={{ width: '8px', height: '8px', borderRadius: '50%', background: getPriorityColor(task.priority) }} />
                      <span style={{ textTransform: 'capitalize', fontSize: '0.85rem' }}>{task.priority}</span>
                    </div>
                  </td>
                  <td>
                    {task.assigned_node ? (
                      <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>{task.assigned_node.split('-')[0]}..</span>
                    ) : (
                      <span style={{ fontSize: '0.85rem', color: 'var(--text-tertiary)' }}>-</span>
                    )}
                  </td>
                  <td>
                    {task.exit_code !== undefined && task.exit_code !== -1 ? (
                      <span style={{ color: task.exit_code === 0 ? 'var(--success)' : 'var(--danger)', fontWeight: 500 }}>
                        {task.exit_code}
                      </span>
                    ) : '-'}
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: '8px' }}>
                      <button className="btn-ghost" title="View Logs" style={{ padding: '6px', borderRadius: '6px' }}>
                        <Terminal size={16} />
                      </button>
                      {task.status === 'running' || task.status === 'queued' ? (
                        <button className="btn-ghost" title="Cancel Task" style={{ padding: '6px', borderRadius: '6px', color: 'var(--warning)' }}>
                          <Square size={16} />
                        </button>
                      ) : (task.status === 'failed' || task.status === 'cancelled') ? (
                        <button className="btn-ghost" title="Retry Task" style={{ padding: '6px', borderRadius: '6px', color: 'var(--primary)' }}>
                          <RotateCcw size={16} />
                        </button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default Tasks;
