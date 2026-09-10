import { useState } from 'react';
import { Form, Input, Button, message, Space } from 'antd';
import { Link, useNavigate as useRouterNavigate } from 'react-router-dom';
import AuthLayout from '../../layouts/AuthLayout';
import SmsCodeButton from '../../components/SmsCodeButton';
import { smsLogin } from '../../api/auth';
import type { SmsLoginReq } from '../../types/auth';

// 手机号 + 短信验证码登录
//  - 点击"获取验证码"按钮触发 SmsCodeButton(60s 倒计时)
//  - sms_code 字段 6 位数字
export default function SmsLoginPage() {
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm<SmsLoginReq>();
  const router = useRouterNavigate();

  const onFinish = async (values: SmsLoginReq) => {
    setLoading(true);
    try {
      await smsLogin(values);
      message.success('登录成功');
      router('/');
    } catch {
      // 拦截器已处理
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthLayout title="短信登录">
      <Form<SmsLoginReq>
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

        <Form.Item shouldUpdate noStyle>
          {() => (
            <Form.Item
              name="sms_code"
              label="验证码"
              rules={[
                { required: true, message: '请输入验证码' },
                { len: 6, message: '验证码为 6 位数字' },
              ]}
            >
              <Space.Compact style={{ width: '100%' }}>
                <Input placeholder="6 位短信验证码" maxLength={6} />
                <SmsCodeButton
                  phone={form.getFieldValue('phone')}
                  onError={(msg) => message.error(msg)}
                />
              </Space.Compact>
            </Form.Item>
          )}
        </Form.Item>

        <Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            登录
          </Button>
        </Form.Item>
      </Form>

      <div style={{ textAlign: 'center' }}>
        <Link to="/login">返回密码登录</Link>
      </div>
    </AuthLayout>
  );
}