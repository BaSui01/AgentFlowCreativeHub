import { Navigate, useLocation } from 'react-router-dom';
import { Spin } from 'antd';

import { useAuth } from '@/features/auth/model/auth-context';

interface ProtectedRouteProps {
	children: React.ReactElement;
}

export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ children }) => {
	const location = useLocation();
	const { isAuthenticated, isLoading } = useAuth();

	if (isLoading) {
		return (
			<div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh' }}>
				<Spin size="large" tip="加载中" />
			</div>
		);
	}

	if (!isAuthenticated) {
		return <Navigate to="/auth/login" replace state={{ from: location }} />;
	}

	return children;
};
