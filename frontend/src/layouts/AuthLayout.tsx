import { ReactNode } from 'react';
import { Card, Typography } from 'antd';

// AuthLayout:登录 / 注册 / 忘记密码 共用的卡片居中布局
//  - 全屏居中卡片
//  - 顶部标题
//  - children 区域放表单
export interface AuthLayoutProps {
  title: string;
  children: ReactNode;
}

export default function AuthLayout({ title, children }: AuthLayoutProps) {
  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#f0f2f5',
        padding: 24,
      }}
    >
      <Card style={{ width: 400, boxShadow: '0 2px 8px rgba(0,0,0,0.08)' }}>
        <Typography.Title level={3} style={{ textAlign: 'center', marginBottom: 32 }}>
          {title}
        </Typography.Title>
        {children}
      </Card>
    </div>
  );
}