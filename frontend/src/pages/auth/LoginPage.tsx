import React from 'react';
import { Alert, Card, Form, Input, Button, Typography } from 'antd';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';

import { useAuth } from '@/features/auth/model/auth-context';

const { Title, Paragraph } = Typography;

type LocationState = {
  from?: {
    pathname: string;
  };
};

export const LoginPage: React.FC = () => {
  const { login, isLoading, error, isAuthenticated } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const resolveRedirectPath = () => {
    return (location.state as LocationState | null)?.from?.pathname ?? '/dashboard';
  };

  if (isAuthenticated) {
    return <Navigate to={resolveRedirectPath()} replace />;
  }

  const onFinish = async (values: { email: string; password: string }) => {
    try {
      await login(values);
      navigate(resolveRedirectPath(), { replace: true });
    } catch {
      // 错误已在 AuthProvider 中提示，这里无需重复处理
    }
  };

  return (
    <Card style={{ width: 420 }}>
      <Title level={3}>欢迎回来</Title>
      <Paragraph type="secondary">使用邮箱和密码登录 AgentFlow Creative Hub</Paragraph>

      {error && (
        <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />
      )}

      <Form layout="vertical" onFinish={onFinish} requiredMark={false}>
        <Form.Item
          label="邮箱"
          name="email"
          rules={[
            { required: true, message: '请输入邮箱地址' },
            { type: 'email', message: '邮箱格式不正确' },
          ]}
        >
          <Input size="large" placeholder="name@example.com" autoComplete="email" />
        </Form.Item>

        <Form.Item
          label="密码"
          name="password"
          rules={[{ required: true, message: '请输入密码' }]}
        >
          <Input.Password size="large" placeholder="请输入密码" autoComplete="current-password" />
        </Form.Item>

        <Button type="primary" htmlType="submit" size="large" block loading={isLoading}>
          登录
        </Button>
      </Form>
    </Card>
  );
};
