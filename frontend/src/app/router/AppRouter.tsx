import { createBrowserRouter, RouterProvider, Navigate } from 'react-router-dom';
import { MainLayout } from '@/shared/ui/layouts/MainLayout';
import { AuthLayout } from '@/shared/ui/layouts/AuthLayout';
import { DashboardPage } from '@/pages/dashboard/DashboardPage';
import { LoginPage } from '@/pages/auth/LoginPage';
import { ProtectedRoute } from '@/shared/ui/ProtectedRoute';
import { PermissionGuard } from '@/shared/ui/PermissionGuard';
import { PERMISSIONS } from '@/features/auth/model/use-authorization';
import RoleManagementPage from '@/pages/admin/RoleManagementPage';
import OperationsCenterPage from '@/pages/admin/OperationsCenterPage';

const router = createBrowserRouter([
  {
    path: '/',
    element: (
      <ProtectedRoute>
        <MainLayout />
      </ProtectedRoute>
    ),
    children: [
      {
        path: 'dashboard',
        element: <DashboardPage />,
      },
      {
        path: 'admin/roles',
        element: (
          <PermissionGuard
            requiredPermissions={[PERMISSIONS.MANAGE_ROLES]}
            fallback={<Navigate to="/dashboard" replace />}
          >
            <RoleManagementPage />
          </PermissionGuard>
        ),
      },
      {
        path: 'admin/operations',
        element: (
          <PermissionGuard
            requiredPermissions={[PERMISSIONS.WORKSPACE_REVIEW, PERMISSIONS.COMMAND_EXECUTE]}
            fallback={<Navigate to="/dashboard" replace />}
          >
            <OperationsCenterPage />
          </PermissionGuard>
        ),
      },
      {
        path: '/',
        element: <Navigate to="/dashboard" replace />,
      },
    ],
  },
  {
    path: '/auth',
    element: <AuthLayout />,
    children: [
      {
        path: 'login',
        element: <LoginPage />,
      },
    ],
  },
]);

export const AppRouter = () => {
  return <RouterProvider router={router} />;
};
