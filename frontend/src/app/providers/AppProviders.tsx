import React, { useEffect } from 'react';
import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { AuthProvider } from '@/features/auth/model/auth-context';
import { OpenAPI } from '@/shared/api/core/OpenAPI';

const queryClient = new QueryClient();
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:7000';

interface AppProvidersProps {
  children: React.ReactNode;
}

export const AppProviders: React.FC<AppProvidersProps> = ({ children }) => {
  useEffect(() => {
    OpenAPI.BASE = API_BASE_URL;
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <ConfigProvider>
        <AuthProvider>
          {children}
        </AuthProvider>
      </ConfigProvider>
    </QueryClientProvider>
  );
};
