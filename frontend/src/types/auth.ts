// 与后端 Go DTO 对齐的类型定义(backend/internal/model/account/dto.go)

// 后端统一响应格式
export interface ApiResp<T = unknown> {
  code: number;
  msg: string;
  data: T;
}

// POST /api/v1/auth/register 请求体
export interface RegisterReq {
  user_name: string;
  password: string;
  phone: string;
  email?: string;
}

// POST /api/v1/auth/login 请求体
export interface LoginReq {
  phone: string;
  password: string;
}

// POST /api/v1/auth/login (SMS 变体)请求体
export interface SmsLoginReq {
  phone: string;
  sms_code: string;
}

// PUT /api/v1/auth/password 请求体(忘记密码/改密)
export interface ChangePasswordReq {
  phone: string;
  sms_code: string;
  password: string;
}

// POST /api/v1/auth/sms-code 请求体
export interface SendSmsCodeReq {
  phone: string;
}

// 后端 SMS 响应(dev 模式会把验证码放进 data.sms_code)
export interface SendSmsCodeResp {
  sms_code?: string;
}

// 登录成功响应
export interface LoginSuccessResp {
  logged_in: boolean;
}

// 后端错误码常量(对应 errs.ServiceErr.Code)
export const ErrCode = {
  InvalidParam: 40001,
  Unauthorized: 40101,
  NotFound: 40401,
  Conflict: 40901,
  TooFrequent: 42901,
  Internal: 50001,
} as const;