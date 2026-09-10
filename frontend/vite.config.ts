import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Vite 配置:
//  - React 插件
//  - 开发服务器:把所有 /api 请求代理到后端 (默认 http://localhost:8080)
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});