import axios, { AxiosError, type AxiosRequestConfig, type AxiosResponse } from 'axios';
import type { MessageInstance } from 'antd/es/message/interface';
import type { ApiResp } from '../types/auth';

// 全局 message 实例:本模块是模块级代码、不在 React 组件树内,用不了 App.useApp(),
// 由根组件 App.tsx 挂载时注入。好处是能读到 ConfigProvider 的主题/语言包(静态 message 读不到)。
let globalMessage: MessageInstance | null = null;

export function setGlobalMessage(instance: MessageInstance) {
  globalMessage = instance;
}

// baseURL 用相对路径,开发环境由 vite.config.ts 代理到后端;
// withCredentials 让后端 SetTokenCookies 的 cookie 跨端口生效。
const http = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  withCredentials: true,
});

// 拦截器解包后端统一格式 {code, msg, data}:成功返回 data,失败弹错并 reject
http.interceptors.response.use(
  (resp: AxiosResponse<ApiResp>) => {
    const body = resp.data;
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) {
        return body.data as unknown as AxiosResponse;
      }
      globalMessage?.error(body.msg || '请求失败');
      return Promise.reject(new Error(body.msg || `code=${body.code}`));
    }
    // 兼容没有 code 的响应
    return resp;
  },
  (err: AxiosError<ApiResp>) => {
    const msg = err.response?.data?.msg || err.message || '网络错误';
    globalMessage?.error(msg);
    return Promise.reject(err);
  },
);

// 返回业务数据 T。axios 拦截器的类型签名要求"进什么类型出什么类型",但这里已把
// {code, msg, data} 解包成 data,与 AxiosResponse 不符;把 as 强转收拢在此函数,
// 调用方(api/*.ts)才能拿到诚实的 T,避免"类型是 AxiosResponse、运行时是业务数据"的错位扩散。
export function request<T>(config: AxiosRequestConfig): Promise<T> {
  return http.request(config) as unknown as Promise<T>;
}

export default http;