import { request } from './axios';
import type {
  ChangePasswordReq,
  LoginReq,
  LoginSuccessResp,
  RegisterReq,
  SendSmsCodeReq,
  SendSmsCodeResp,
  SmsLoginReq,
  UserResp,
} from '../types/auth';

// 所有账号相关 API,路径对齐 backend/internal/model/account/router.go
//
// 注意:
//  - token 由后端通过 HttpOnly cookie 下发,前端不存 localStorage
//    (withCredentials 已在 axios 实例里统一开启)
//  - request<T> 的返回值就是业务数据 T(拦截器已解包 {code,msg,data}),
//    不是 AxiosResponse,所以不要访问 .data / .status

// POST /auth/register 注册
export function register(req: RegisterReq): Promise<LoginSuccessResp> {
  return request<LoginSuccessResp>({ url: '/auth/register', method: 'post', data: req });
}

// POST /auth/login 手机号 + 密码登录
export function login(req: LoginReq): Promise<LoginSuccessResp> {
  return request<LoginSuccessResp>({ url: '/auth/login', method: 'post', data: req });
}

// POST /auth/login 手机号 + 短信验证码登录(复用 login 接口,sms_code 走 service 分支)
export function smsLogin(req: SmsLoginReq): Promise<LoginSuccessResp> {
  return request<LoginSuccessResp>({ url: '/auth/login', method: 'post', data: req });
}

// POST /auth/logout 登出(后端从 cookie 读 token)
export function logout(): Promise<{ logged_out: boolean }> {
  return request<{ logged_out: boolean }>({ url: '/auth/logout', method: 'post' });
}

// GET /users/me 取当前登录用户资料(需登录,token 由 cookie 携带)
export function getMyProfile(): Promise<UserResp> {
  return request<UserResp>({ url: '/users/me', method: 'get' });
}

// POST /auth/sms-code 发送短信验证码
// 返回 SendSmsCodeResp:dev 模式下 data.sms_code 会有值,方便调试
export function sendSmsCode(req: SendSmsCodeReq): Promise<SendSmsCodeResp> {
  return request<SendSmsCodeResp>({ url: '/auth/sms-code', method: 'post', data: req });
}

// PUT /auth/password 修改密码(SMS 验证码流程,无需登录态)
export function changePassword(req: ChangePasswordReq): Promise<null> {
  return request<null>({ url: '/auth/password', method: 'put', data: req });
}
