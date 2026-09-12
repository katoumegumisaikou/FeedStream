import { useState } from 'react';
import { App, Form, Input, Button, Divider } from 'antd';
import { Link, useNavigate as useRouterNavigate } from 'react-router-dom';
import AuthLayout from '../../layouts/AuthLayout';
import { login } from '../../api/auth';
import type { LoginReq } from '../../types/auth';

// 手机号 + 密码登录
//  - phone:11 位中国手机号
//  - password:至少 6 位
// 成功后由后端 SetTokenCookies 写入 access_token / refresh_token
export default function LoginPage() {
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm<LoginReq>();
  const router = useRouterNavigate();
  const { message } = App.useApp();

  const onFinish = async (values: LoginReq) => {
    setLoading(true);
    try {
      await login(values);
      message.success('登录成功');
      router('/'); // 登录成功后跳首页(暂未实现,先跳根路径)
    } catch {
      // 拦截器已弹错
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthLayout title="登录">
      <Form<LoginReq>
        form={form}
          layout="vertical"
          onFinish={onFinish}
          autoComplete="off"
        >
        <Form.Item
          name="phone"
          label="手机号"
          rules={[
            { required: true, message: '请输入手机号' },
            { pattern: /^1[3-9]\d{9}$/, message: '手机号格式不正确' },
          ]}
        >
          <Input placeholder="请输入 11 位手机号" maxLength={11} />
        </Form.Item>

        <Form.Item
          name="password"
          label="密码"
          rules={[
            { required: true, message: '请输入密码' },
            { min: 8, max: 16, message: '密码 8-16 位' },
          ]}
        >
          <Input.Password placeholder="请输入密码" />
        </Form.Item>

        <Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            登录
          </Button>
        </Form.Item>
      </Form>

      <Divider style={{ margin: '12px 0' }} />

      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
        <Link to="/login/sms">短信验证码登录</Link>
        <Link to="/forgot-password">忘记密码</Link>
      </div>

      <div style={{ textAlign: 'center', marginTop: 16 }}>
        还没有账号?<Link to="/register">立即注册</Link>
      </div>
    </AuthLayout>
  );
}