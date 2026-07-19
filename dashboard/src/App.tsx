import React from 'react';
import { BrowserRouter, Routes, Route, NavLink, useLocation } from 'react-router-dom';
import { LayoutDashboard, Server, ListTodo, Settings, Cpu } from 'lucide-react';
import './index.css';

// Lazy load pages
import Dashboard from './pages/Dashboard';
import Nodes from './pages/Nodes';
import Tasks from './pages/Tasks';

const Sidebar = ({ onLogout, username }: { onLogout: () => void, username: string | null }) => {
  const navItems = [
    { name: 'Overview', path: '/', icon: <LayoutDashboard size={20} /> },
    { name: 'Nodes', path: '/nodes', icon: <Server size={20} /> },
    { name: 'Tasks', path: '/tasks', icon: <ListTodo size={20} /> },
    { name: 'Resources', path: '/resources', icon: <Cpu size={20} /> },
    { name: 'Settings', path: '/settings', icon: <Settings size={20} /> },
  ];

  return (
    <aside className="sidebar">
      <div style={{ padding: '32px 24px', display: 'flex', alignItems: 'center', gap: '12px' }}>
        <div style={{
          width: '32px', height: '32px', borderRadius: '8px',
          background: 'var(--primary)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          boxShadow: 'var(--shadow-sm)'
        }}>
          <span style={{ color: 'white', fontWeight: 'bold', fontSize: '18px' }}>C</span>
        </div>
        <h2 style={{ fontSize: '1.25rem', margin: 0, color: 'var(--text-primary)' }}>Constellation</h2>
      </div>

      <nav style={{ padding: '0 16px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {navItems.map((item) => (
          <NavLink
            key={item.name}
            to={item.path}
            style={({ isActive }) => ({
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '12px 16px',
              borderRadius: 'var(--radius-md)',
              color: isActive ? 'var(--primary)' : 'var(--text-secondary)',
              background: isActive ? 'var(--bg-panel-hover)' : 'transparent',
              textDecoration: 'none',
              transition: 'all var(--transition-fast)',
              fontWeight: isActive ? 500 : 400,
            })}
            className={({ isActive }) => isActive ? 'nav-active' : 'nav-item'}
          >
            <span style={{ color: 'inherit' }}>{item.icon}</span>
            {item.name}
          </NavLink>
        ))}
      </nav>
      
      <div style={{ marginTop: 'auto', padding: '24px', borderTop: '1px solid var(--border-color)', display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <div style={{ width: '8px', height: '8px', borderRadius: '50%', background: 'var(--success)' }} />
          <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Controller Online</span>
        </div>
        
        {username && (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: '0.85rem', fontWeight: 500 }}>{username}</span>
            <button 
              onClick={onLogout}
              style={{
                background: 'none', border: 'none', cursor: 'pointer',
                color: 'var(--text-secondary)', fontSize: '0.8rem',
                textDecoration: 'underline'
              }}
            >
              Logout
            </button>
          </div>
        )}
      </div>
    </aside>
  );
};

import Login from './pages/Login';

const App = () => {
  const [token, setToken] = React.useState<string | null>(localStorage.getItem('constellation_token'));
  const [username, setUsername] = React.useState<string | null>(localStorage.getItem('constellation_username'));

  const handleLogin = (newToken: string, newUsername: string) => {
    localStorage.setItem('constellation_token', newToken);
    localStorage.setItem('constellation_username', newUsername);
    setToken(newToken);
    setUsername(newUsername);
  };

  const handleLogout = () => {
    localStorage.removeItem('constellation_token');
    localStorage.removeItem('constellation_username');
    setToken(null);
    setUsername(null);
  };

  if (!token) {
    return <Login onLogin={handleLogin} />;
  }

  return (
    <BrowserRouter>
      <div className="app-container">
        <Sidebar onLogout={handleLogout} username={username} />
        <main className="main-content animate-fade-in">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/nodes" element={<Nodes />} />
            <Route path="/tasks" element={<Tasks />} />
            {/* Fallbacks for uncompleted pages */}
            <Route path="*" element={
              <div className="glass-panel" style={{ padding: '40px', textAlign: 'center' }}>
                <h2 style={{ marginBottom: '16px' }}>Coming Soon</h2>
                <p style={{ color: 'var(--text-secondary)' }}>This module is currently under development.</p>
              </div>
            } />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  );
};

export default App;
