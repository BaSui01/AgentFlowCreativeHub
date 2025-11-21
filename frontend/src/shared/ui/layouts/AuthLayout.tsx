import React from 'react';
import { Layout, Typography } from 'antd';
import { Outlet } from 'react-router-dom';

const { Content, Sider } = Layout;

export const AuthLayout: React.FC = () => {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider breakpoint="lg" collapsedWidth={0} style={{ background: 'linear-gradient(135deg, #111b2b 0%, #1f3a93 100%)', color: '#fff', padding: '48px 32px' }}>
        <Typography.Title level={2} style={{ color: '#fff' }}>AgentFlow</Typography.Title>
        <Typography.Paragraph style={{ color: 'rgba(255,255,255,0.85)' }}>
          多租户智能代理协作平台
        </Typography.Paragraph>
      </Sider>
      <Content style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', padding: '32px' }}>
        <Outlet />
      </Content>
    </Layout>
  );
};
