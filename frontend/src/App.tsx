import { useEffect } from 'react';
import { App as AntdApp } from 'antd';
import { AppRouter } from './router';
import { setGlobalMessage } from './api/axios';

// 把 antd 的 message 注入 axios:拦截器是模块级代码用不了 App.useApp(),只能这样桥接;
// 依赖 main.tsx 的 <AntdApp> 包裹,缺了 useApp() 拿不到上下文。
export default function App() {
  const { message } = AntdApp.useApp();

  useEffect(() => {
    setGlobalMessage(message);
  }, [message]);

  return <AppRouter />;
}
