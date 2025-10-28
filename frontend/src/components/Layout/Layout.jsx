import React, { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import {
  HomeIcon,
  ChartBarIcon,
  MagnifyingGlassIcon,
  Cog6ToothIcon,
  ArrowRightOnRectangleIcon,
  Bars3Icon,
  XMarkIcon,
} from '@heroicons/react/24/outline';

import './Layout.css';

const Layout = ({ children }) => {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const { user, logout } = useAuth();
  const location = useLocation();

  const navigation = [
    { name: 'Dashboard', href: '/dashboard', icon: HomeIcon },
    { name: 'AI Query', href: '/query', icon: ChartBarIcon },
    { name: 'Data Explorer', href: '/explore', icon: MagnifyingGlassIcon },
    { name: 'Settings', href: '/settings', icon: Cog6ToothIcon },
  ];

  const handleLogout = () => {
    logout();
  };

  return (
    <div className="layout">
      {/* Mobile sidebar overlay */}
      {sidebarOpen && (
        <div
          className="sidebar-overlay"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <div className={`sidebar ${sidebarOpen ? 'sidebar-open' : ''}`}>
        <div className="sidebar-header">
          <div className="logo-container">
            <div className="logo">
              <span className="logo-text">AI DB</span>
            </div>
          </div>

          <button
            className="sidebar-close-btn lg:hidden"
            onClick={() => setSidebarOpen(false)}
          >
            <XMarkIcon className="h-6 w-6" />
          </button>
        </div>

        <nav className="sidebar-nav">
          {navigation.map((item) => {
            const isActive = location.pathname === item.href;
            return (
              <Link
                key={item.name}
                to={item.href}
                className={`nav-item ${isActive ? 'nav-item-active' : ''}`}
                onClick={() => setSidebarOpen(false)}
              >
                <item.icon className="nav-icon" />
                <span>{item.name}</span>
              </Link>
            );
          })}
        </nav>

        <div className="sidebar-footer">
          <div className="user-info">
            <div className="user-avatar">
              {user?.name?.charAt(0).toUpperCase()}
            </div>
            <div className="user-details">
              <p className="user-name">{user?.name}</p>
              <p className="user-email">{user?.email}</p>
            </div>
          </div>

          <button
            onClick={handleLogout}
            className="logout-btn"
          >
            <ArrowRightOnRectangleIcon className="h-5 w-5" />
            <span>Logout</span>
          </button>
        </div>
      </div>

      {/* Main content */}
      <div className="main-content">
        <header className="header">
          <button
            className="sidebar-toggle lg:hidden"
            onClick={() => setSidebarOpen(true)}
          >
            <Bars3Icon className="h-6 w-6" />
          </button>

          <div className="header-title">
            <h1>AI Database Application</h1>
          </div>
        </header>

        <main className="content">
          {children}
        </main>
      </div>
    </div>
  );
};

export default Layout;
