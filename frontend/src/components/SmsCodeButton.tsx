import { useEffect, useRef, useState } from 'react';
import { Button } from 'antd';
import { sendSmsCode } from '../api/auth';

// SmsCodeButton:发送短信验证码 + 60s 倒计时
//  - 父组件通过 phone 传入手机号
//  - 倒计时未归零前按钮 disabled,显示 "Xs 后重试"
//  - 倒计时归零自动恢复可点击
//
// 用法:
//   <SmsCodeButton phone={phone} onError={(e) => message.error(e)} />
export interface SmsCodeButtonProps {
  phone: string;
  onError?: (msg: string) => void;
}

const COUNTDOWN_SECONDS = 60;

export default function SmsCodeButton({ phone, onError }: SmsCodeButtonProps) {
  const [loading, setLoading] = useState(false);
  const [remaining, setRemaining] = useState(0);
  const timerRef = useRef<number | null>(null);

  // 卸载时清 timer,避免内存泄漏 / state 更新 on unmounted 组件
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        window.clearInterval(timerRef.current);
      }
    };
  }, []);

  const startCountdown = () => {
    setRemaining(COUNTDOWN_SECONDS);
    timerRef.current = window.setInterval(() => {
      setRemaining((prev) => {
        if (prev <= 1) {
          if (timerRef.current) {
            window.clearInterval(timerRef.current);
            timerRef.current = null;
          }
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  };

  const handleClick = async () => {
    if (!phone || phone.length < 11) {
      onError?.('请先填写正确的手机号');
      return;
    }
    setLoading(true);
    try {
      await sendSmsCode({ phone });
      startCountdown();
    } catch {
      // axios 拦截器已经弹过 message,这里不重复提示
    } finally {
      setLoading(false);
    }
  };

  const disabled = loading || remaining > 0;
  const text = remaining > 0 ? `${remaining}s 后重试` : '获取验证码';

  return (
    <Button disabled={disabled} loading={loading} onClick={handleClick}>
      {text}
    </Button>
  );
}