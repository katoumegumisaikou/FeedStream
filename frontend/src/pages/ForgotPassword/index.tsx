import { useState } from 'react';
import { App, Form, Input, Button, Space } from 'antd';
import { Link, useNavigate as useRouterNavigate } from 'react-router-dom';
import AuthLayout from '../../layouts/AuthLayout';
import SmsCodeButton from '../../components/SmsCodeButton';
import { changePassword } from '../../api/auth';
import type { ChangePasswordReq } from '../../types/auth';

// 忘记密码(走 SMS 验证码流程,无需登录态)
//  - phone + sms_code + new_password
//  - 后端校验通过后:递增 user.Version → 旧 token 失效 → 发放新 token 并 SetTokenCookies
export default function ForgotPasswordPage() {
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm<ChangePasswordReq>();
  const router = useRouterNavigate();
  const { message } = App.useApp();

  const onFinish = async (values: ChangePasswordReq) => {
    setLoading(true);
    try {
      await changePassword(values);
      message.success('密码已重置,已自动登录');
      router('/');
    } catch {
      // 拦截器已处理
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthLayout title="忘记密码">
      <Form<ChangePasswordReq>
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
          <Input placeholder="11 位手机号" maxLength={11} />
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

        <Form.Item
          name="password"
          label="新密码"
          rules={[
            { required: true, message: '请输入新密码' },
            { min: 8, max: 16, message: '密码 8-16 位' },
            {
              validator: async (_, value: string) => {
                if (!value) return Promise.resolve();
                // 与后端 password.Strong 对齐:只允许 ASCII 可打印字符(排除空格)
                if (!/^[\x21-\x7E]+$/.test(value)) {
                  return Promise.reject(new Error('密码只能使用字母、数字和常见符号'));
                }
                const types = [/[a-zA-Z]/.test(value), /\d/.test(value), /[^a-zA-Z0-9]/.test(value)];
                const cnt = types.filter(Boolean).length;
                return cnt >= 2 ? Promise.resolve() : Promise.reject(new Error('需含字母、数字、特殊字符中的至少两类'));
              },
            },
          ]}
          hasFeedback
        >
          <Input.Password placeholder="字母+数字+特殊字符,至少 2 类" />
        </Form.Item>

        <Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            重置密码
          </Button>
        </Form.Item>
      </Form>

      <div style={{ textAlign: 'center' }}>
        想起密码了?<Link to="/login">返回登录</Link>
      </div>
    </AuthLayout>
  );
}