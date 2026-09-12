import { useEffect } from 'react';
import { App as AntdApp } from 'antd';
import { AppRouter } from './router';
import { setGlobalMessage } from './api/axios';

// App 作为根组件:
//  - 渲染路由
//  - 把 antd 的 message 实例注入 axios
//    (axios 拦截器是模块级代码,用不了 App.useApp(),只能这样桥接)
//  - 依赖 main.tsx 里的 <AntdApp> 包裹,否则 useApp() 拿不到上下文
export default function App() {
  const { message } = AntdApp.useApp();

  useEffect(() => {
    setGlobalMessage(message);
  }, [message]);

  return <AppRouter />;
}
