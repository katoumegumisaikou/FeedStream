import { Navigate, Route, Routes } from 'react-router-dom';
import LoginPage from './pages/Login';
import SmsLoginPage from './pages/SmsLogin';
import RegisterPage from './pages/Register';
import ForgotPasswordPage from './pages/ForgotPassword';

// 路由表:
//  - /login             手机号 + 密码登录(默认入口)
//  - /login/sms         手机号 + 短信验证码登录
//  - /register          注册
//  - /forgot-password   忘记密码(SMS 验证码流程)
//  - /                  重定向到 /login
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