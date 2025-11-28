import React, { useMemo } from 'react';
import { Layout, Menu, Avatar, Dropdown, Button, Typography, Space } from 'antd';
import { DashboardOutlined, HomeOutlined, LogoutOutlined, TeamOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

import { useAuth } from '@/features/auth/model/auth-context';
import { PERMISSIONS, useAuthorization } from '@/features/auth/model/use-authorization';

const { Header, Content, Footer, Sider } = Layout;

export const MainLayout: React.FC = () => {
  const { user, logout } = useAuth();
  const { hasPermission } = useAuthorization();
  const location = useLocation();
  const navigate = useNavigate();

  const selectedKey = useMemo(() => {
    if (location.pathname.startsWith('/admin/roles')) {
      return '/admin/roles';
    }
    if (location.pathname.startsWith('/admin/operations')) {
      return '/admin/operations';
    }
    return '/dashboard';
  }, [location.pathname]);

  const menuItems = useMemo(() => {
    const items = [
      {
        key: '/dashboard',
        icon: <HomeOutlined />,
        label: '仪表盘',
        onClick: () => navigate('/dashboard'),
      },
    ];
    if (hasPermission(PERMISSIONS.WORKSPACE_REVIEW) || hasPermission(PERMISSIONS.COMMAND_ADMIN)) {
      items.push({
        key: '/admin/operations',
        icon: <DashboardOutlined />,
        label: '运营控制台',
        onClick: () => navigate('/admin/operations'),
      });
    }
    if (hasPermission(PERMISSIONS.MANAGE_ROLES)) {
      items.push({
        key: '/admin/roles',
        icon: <TeamOutlined />,
        label: '角色与权限',
        onClick: () => navigate('/admin/roles'),
      });
    }
    return items;
  }, [hasPermission, navigate]);

  const userMenu = {
    items: [
      {
        key: 'profile',
        label: (
          <div style={{ minWidth: 200 }}>
            <Typography.Text strong>{user?.name ?? '未命名用户'}</Typography.Text>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {user?.email ?? '未绑定邮箱'}
            </Typography.Paragraph>
          </div>
        ),
        disabled: true,
      },
      {
        type: 'divider' as const,
      },
      {
        key: 'logout',
        icon: <LogoutOutlined />,
        label: '退出登录',
        onClick: logout,
      },
    ],
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider breakpoint="lg" collapsedWidth={64}>
        <div style={{ height: 48, margin: 16, display: 'flex', alignItems: 'center', gap: 8, color: '#fff', fontWeight: 600 }}>
          <ThunderboltOutlined /> AF Hub
        </div>
        <Menu theme="dark" mode="inline" selectedKeys={[selectedKey]} items={menuItems} />
      </Sider>
      <Layout>
        <Header style={{ padding: '0 24px', background: '#fff', display: 'flex', justifyContent: 'flex-end', alignItems: 'center', gap: 16 }}>
          <Dropdown menu={userMenu} placement="bottomRight" arrow trigger={["click"]}>
            <Button type="text">
              <Space size={8}>
                <Avatar>
                  {(user?.name ?? user?.email ?? 'U').charAt(0).toUpperCase()}
                </Avatar>
                <span>{user?.name ?? user?.email ?? '用户'}</span>
              </Space>
            </Button>
          </Dropdown>
        </Header>
        <Content style={{ margin: '0 16px' }}>
          <div style={{ padding: 24, minHeight: 360, background: '#fff', marginTop: 16 }}>
            <Outlet />
          </div>
        </Content>
        <Footer style={{ textAlign: 'center' }}>
          AgentFlow Creative Hub ©2025
        </Footer>
      </Layout>
    </Layout>
  );
};
