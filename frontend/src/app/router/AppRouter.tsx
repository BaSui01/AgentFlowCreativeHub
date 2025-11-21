import { createBrowserRouter, RouterProvider, Navigate } from 'react-router-dom';
import { MainLayout } from '@/shared/ui/layouts/MainLayout';
import { AuthLayout } from '@/shared/ui/layouts/AuthLayout';
import { DashboardPage } from '@/pages/dashboard/DashboardPage';
import { LoginPage } from '@/pages/auth/LoginPage';
import { ProtectedRoute } from '@/shared/ui/ProtectedRoute';

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
