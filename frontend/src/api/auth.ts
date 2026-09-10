import http from './axios';
import type {
  ChangePasswordReq,
  LoginReq,
  LoginSuccessResp,
  RegisterReq,
  SendSmsCodeReq,
  SendSmsCodeResp,
  SmsLoginReq,
} from '../types/auth';

// 所有账号相关 API,路径对齐 backend/internal/model/account/router.go
//
// 注意:token 由后端通过 HttpOnly cookie 下发,前端不存 localStorage,
// withCredentials 已在 axios 实例里统一开启

// POST /auth/register 注册
export function register(req: RegisterReq) {
  return http.post<LoginSuccessResp>('/auth/register', req);
}

// POST /auth/login 手机号 + 密码登录
export function login(req: LoginReq) {
  return http.post<LoginSuccessResp>('/auth/login', req);
}

// POST /auth/login 手机号 + 短信验证码登录(复用 login 接口,sms_code 走 service 分支)
export function smsLogin(req: SmsLoginReq) {
  return http.post<LoginSuccessResp>('/auth/login', req);
}

// POST /auth/logout 登出(后端从 cookie 读 token)
export function logout() {
  return http.post<{ logged_out: boolean }>('/auth/logout');
}

// POST /auth/sms-code 发送短信验证码
// 返回 SendSmsCodeResp:dev 模式下 data.sms_code 会有值,方便调试
export function sendSmsCode(req: SendSmsCodeReq) {
  return http.post<SendSmsCodeResp>('/auth/sms-code', req);
}

// PUT /auth/password 修改密码(SMS 验证码流程,无需登录态)
export function changePassword(req: ChangePasswordReq) {
  return http.post<null>('/auth/password', req);
}