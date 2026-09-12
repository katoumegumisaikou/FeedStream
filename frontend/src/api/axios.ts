import axios, { AxiosError, type AxiosRequestConfig, type AxiosResponse } from 'axios';
import type { MessageInstance } from 'antd/es/message/interface';
import type { ApiResp } from '../types/auth';

// 全局 message 实例:
//  - 本文件是模块级代码,不在 React 组件树内,用不了 App.useApp()
//  - 由根组件 App.tsx 在挂载时注入(见 setGlobalMessage 调用处)
//  - 好处:能读到 ConfigProvider 的主题 / 语言包配置(静态 message 读不到)
let globalMessage: MessageInstance | null = null;

// setGlobalMessage 供根组件调用,把上下文感知的 message 实例交给本模块
export function setGlobalMessage(instance: MessageInstance) {
  globalMessage = instance;
}

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

// request 发起请求,直接返回业务数据 T
//
// 为什么要包一层强转:
//   axios 拦截器的类型签名是 (value: T) => T | Promise<T>,要求"进什么类型出什么类型";
//   而本模块的拦截器把 {code, msg, data} 解包了,实际返回的是 data,与 AxiosResponse 不同。
//   把 as 强转收拢在这一个函数里,调用方(api/*.ts)就能拿到诚实的类型 T,
//   避免"类型说是 AxiosResponse、运行时其实是业务数据"的错位扩散到各处。
export function request<T>(config: AxiosRequestConfig): Promise<T> {
  return http.request(config) as unknown as Promise<T>;
}

export default http;