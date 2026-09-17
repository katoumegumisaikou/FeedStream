import { Navigate, Route, Routes } from 'react-router-dom';
import LoginPage from './pages/Login';
import SmsLoginPage from './pages/SmsLogin';
import RegisterPage from './pages/Register';
import ForgotPasswordPage from './pages/ForgotPassword';

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/login" replace />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/login/sms" element={<SmsLoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/forgot-password" element={<ForgotPasswordPage />} />
      <Route path="*" element={<Navigate to="/login" replace />} />
    </Routes>
  );
}