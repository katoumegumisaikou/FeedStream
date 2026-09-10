import axios, { AxiosError, type AxiosResponse } from 'axios';
import { message } from 'antd';
import type { ApiResp } from '../types/auth';

// Axios 实例:
//  - baseURL 用相对路径 /api,开发环境由 vite.config.ts 代理到后端
//  - withCredentials: true 让后端 SetTokenCookies 写入的 cookie 跨端口生效
//  - 拦截器统一解包 {code, msg, data},失败统一弹 antd message
const http = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  withCredentials: true,
});

// 响应拦截器:把后端统一格式 {code, msg, data} 解包
// 成功:返回 data;失败:弹错并 reject
http.interceptors.response.use(
  (resp: AxiosResponse<ApiResp>) => {
    const body = resp.data;
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) {
        return body.data as unknown as AxiosResponse;
      }
      // 后端业务错误
      message.error(body.msg || '请求失败');
      return Promise.reject(new Error(body.msg || `code=${body.code}`));
    }
    // 兼容没有 code 的响应
    return resp;
  },
  (err: AxiosError<ApiResp>) => {
    const msg = err.response?.data?.msg || err.message || '网络错误';
    message.error(msg);
    return Promise.reject(err);
  },
);

export default http;