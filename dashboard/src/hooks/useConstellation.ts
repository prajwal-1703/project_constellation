import { useState, useEffect, useCallback } from 'react';

const API_BASE = '/api/v1';
const WS_URL = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`;

export interface ClusterStatus {
  cluster: { name: string; id: string; };
  health: string;
  pool: {
    total_nodes: number;
    online_nodes: number;
    total_cpu_cores: number;
    available_cpu_cores: number;
    total_memory_bytes: number;
    available_memory_bytes: number;
  };
  task_stats: {
    running: number;
    queued: number;
    completed: number;
    failed: number;
  };
}

export function useConstellation(token: string | null) {
  const [status, setStatus] = useState<ClusterStatus | null>(null);
  const [nodes, setNodes] = useState<any[]>([]);
  const [tasks, setTasks] = useState<any[]>([]);
  const [connected, setConnected] = useState(false);

  const getHeaders = useCallback(() => {
    const headers: Record<string, string> = {};
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    return headers;
  }, [token]);

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/cluster/status`, { headers: getHeaders() });
      if (res.ok) setStatus(await res.json());
    } catch (e) { console.error("Failed to fetch status", e); }
  }, [getHeaders]);

  const fetchNodes = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/nodes`, { headers: getHeaders() });
      if (res.ok) {
        const data = await res.json();
        setNodes(data.nodes || []);
      }
    } catch (e) { console.error("Failed to fetch nodes", e); }
  }, [getHeaders]);

  const fetchTasks = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/tasks`, { headers: getHeaders() });
      if (res.ok) {
        const data = await res.json();
        setTasks(data.tasks || []);
      }
    } catch (e) { console.error("Failed to fetch tasks", e); }
  }, [getHeaders]);

  useEffect(() => {
    if (!token) return;
    // Initial fetch
    fetchStatus();
    fetchNodes();
    fetchTasks();

    // WebSocket connection
    let ws: WebSocket;
    let reconnectTimer: any;

    const connectWS = () => {
      ws = new WebSocket(WS_URL);
      
      ws.onopen = () => setConnected(true);
      
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          // Auto-refresh data based on event type
          if (msg.type?.includes('node')) fetchNodes();
          if (msg.type?.includes('task')) fetchTasks();
          fetchStatus(); // Always update dashboard metrics
        } catch (e) { }
      };

      ws.onclose = () => {
        setConnected(false);
        reconnectTimer = setTimeout(connectWS, 3000);
      };
    };

    connectWS();

    return () => {
      clearTimeout(reconnectTimer);
      if (ws) ws.close();
    };
  }, [fetchStatus, fetchNodes, fetchTasks]);

  return { status, nodes, tasks, connected, API_BASE, mutate: fetchTasks };
}

export function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}
