import React from 'react';
import { useConstellation, formatBytes } from '../hooks/useConstellation';
import { Server, Cpu, HardDrive, Activity, Clock, Trash2, Power } from 'lucide-react';

const Nodes = () => {
  const { nodes, API_BASE } = useConstellation(localStorage.getItem('constellation_token'));

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'online': return <span className="status-badge status-online">Online</span>;
      case 'offline': return <span className="status-badge status-offline">Offline</span>;
      default: return <span className="status-badge status-queued">{status}</span>;
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1 style={{ fontSize: '2rem', marginBottom: '8px' }}>Compute Nodes</h1>
          <p style={{ color: 'var(--text-secondary)' }}>Manage machines connected to the cluster.</p>
        </div>
        <button className="btn btn-primary">Add Node</button>
      </div>

      <div className="glass-panel table-container">
        <table className="table">
          <thead>
            <tr>
              <th>Hostname / ID</th>
              <th>Status</th>
              <th>Role</th>
              <th>CPU Cores</th>
              <th>Memory</th>
              <th>Load</th>
              <th>Running Tasks</th>
            </tr>
          </thead>
          <tbody>
            {nodes.length === 0 ? (
              <tr>
                <td colSpan={7} style={{ textAlign: 'center', padding: '32px', color: 'var(--text-tertiary)' }}>
                  No nodes connected to cluster.
                </td>
              </tr>
            ) : (
              nodes.map(node => (
                <tr key={node.id}>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                      <div style={{ background: 'var(--bg-panel-hover)', padding: '8px', borderRadius: '8px', border: '1px solid var(--border-color)' }}>
                        <Server size={18} color="var(--text-secondary)" />
                      </div>
                      <div>
                        <div style={{ fontWeight: 500, color: 'var(--text-primary)' }}>{node.hostname}</div>
                        <div style={{ fontSize: '0.75rem', color: 'var(--text-tertiary)' }}>{node.id}</div>
                      </div>
                    </div>
                  </td>
                  <td>{getStatusBadge(node.status)}</td>
                  <td>
                    <span style={{ 
                      background: 'var(--bg-panel-hover)', 
                      border: '1px solid var(--border-color)',
                      padding: '2px 8px', 
                      borderRadius: '4px', 
                      fontSize: '0.8rem',
                      color: 'var(--text-secondary)'
                    }}>
                      {node.role}
                    </span>
                  </td>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <Cpu size={14} color="var(--primary)" />
                      <span>{node.cpu_cores}</span>
                      <span style={{ fontSize: '0.75rem', color: 'var(--text-tertiary)' }}>
                        ({Math.round(node.cpu_usage || 0)}%)
                      </span>
                    </div>
                  </td>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <HardDrive size={14} color="var(--accent)" />
                      <span>{formatBytes(node.memory_total)}</span>
                      <span style={{ fontSize: '0.75rem', color: 'var(--text-tertiary)' }}>
                        ({Math.round(node.memory_usage || 0)}%)
                      </span>
                    </div>
                  </td>
                  <td>{node.load_avg_1?.toFixed(2) || '0.00'}</td>
                  <td>
                    <span style={{ 
                      display: 'inline-flex', 
                      alignItems: 'center', 
                      justifyContent: 'center',
                      background: node.running_tasks > 0 ? 'var(--primary-glow)' : 'transparent',
                      color: node.running_tasks > 0 ? 'var(--primary)' : 'var(--text-tertiary)',
                      border: '1px solid',
                      borderColor: node.running_tasks > 0 ? 'var(--primary)' : 'var(--border-color)',
                      width: '24px', height: '24px', borderRadius: '50%',
                      fontSize: '0.8rem', fontWeight: 600
                    }}>
                      {node.running_tasks || 0}
                    </span>
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

export default Nodes;
