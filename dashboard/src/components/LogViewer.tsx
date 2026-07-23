import { useEffect, useRef, useState } from 'react';

const LogViewer = ({ taskId, onClose }: { taskId: string, onClose: () => void }) => {
  const [logs, setLogs] = useState<string[]>([]);
  const ws = useRef<WebSocket | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const token = localStorage.getItem('constellation_token');
    const url = `${protocol}//${host}/api/v1/tasks/${taskId}/logs/ws?token=${token}`;

    ws.current = new WebSocket(url);

    ws.current.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'log') {
          setLogs(prev => [...prev, data.data.line]);
        }
      } catch (e) {
        console.error("Failed to parse log message", e);
      }
    };

    return () => {
      if (ws.current) ws.current.close();
    };
  }, [taskId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  return (
    <div style={{
      position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
      background: 'rgba(0,0,0,0.8)', backdropFilter: 'blur(4px)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000
    }}>
      <div className="glass-panel animate-fade-in" style={{ width: '80%', maxWidth: '900px', height: '70vh', display: 'flex', flexDirection: 'column' }}>
        <div style={{ padding: '16px 24px', borderBottom: '1px solid var(--border-color)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h3 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
            <div style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--success)', boxShadow: '0 0 8px var(--success)' }} />
            Live Logs: {taskId.substring(0, 8)}
          </h3>
          <button className="btn-ghost" onClick={onClose}>Close</button>
        </div>
        <div style={{ flex: 1, padding: '24px', overflowY: 'auto', background: 'rgba(0,0,0,0.3)', fontFamily: 'monospace', fontSize: '0.9rem', color: '#e5e7eb' }}>
          {logs.map((log, i) => (
            <div key={i} style={{ marginBottom: '4px' }}>
              <span style={{ color: 'var(--text-tertiary)', marginRight: '12px' }}>
                {new Date().toISOString().split('T')[1].split('.')[0]}
              </span>
              {log}
            </div>
          ))}
          {logs.length === 0 && <div style={{ color: 'var(--text-tertiary)' }}>Connecting to agent stream...</div>}
          <div ref={bottomRef} />
        </div>
      </div>
    </div>
  );
};

export default LogViewer;
