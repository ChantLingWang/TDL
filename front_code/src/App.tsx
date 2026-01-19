import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import './styles/global.scss';

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/" element={<Navigate to="/login" replace />} />
        {/* Add other routes here later */}
        <Route path="/dashboard" element={<div style={{ padding: 20, color: '#fff' }}>DASHBOARD PLACEHOLDER</div>} />
      </Routes>
    </Router>
  );
}

export default App;
