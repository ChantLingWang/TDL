import { useEffect, useRef } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { authApi } from './api/auth';
import { expiresWithin } from './utils/token';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import ChatPage from './pages/ChatPage';
import './styles/global.scss';

function App() {
  const refreshTimer = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  useEffect(() => {
    const refresh = async () => {
      const access = localStorage.getItem('access_token');
      const refresh = localStorage.getItem('refresh_token');
      const userInfo = JSON.parse(localStorage.getItem('user_info') || '{}');
      if (!access || !refresh || !userInfo.user_id) return;
      if (!expiresWithin(access, 5)) return;

      try {
        const res = await authApi.refreshToken({ user_id: String(userInfo.user_id), refresh_token: refresh, email: userInfo.email });
        localStorage.setItem('access_token', res.data.access_token);
        localStorage.setItem('refresh_token', res.data.refresh_token);
      } catch {
        localStorage.clear();
        window.location.href = '/login';
      }
    };

    refresh(); // 首次立即检查
    refreshTimer.current = setInterval(refresh, 60_000); // 每分钟检查
    return () => { if (refreshTimer.current) clearInterval(refreshTimer.current); };
  }, []);

  return (
    <Router>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/" element={<Navigate to="/login" replace />} />
        {/* Add other routes here later */}
        <Route path="/dashboard" element={<ChatPage />} />
      </Routes>
    </Router>
  );
}

export default App;
