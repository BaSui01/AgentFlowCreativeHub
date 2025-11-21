import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { message } from 'antd';

import { AuthService, type auth_LoginResponse, type auth_UserInfo } from '@/shared/api';
import { OpenAPI } from '@/shared/api/core/OpenAPI';

type AuthCredentials = {
	email: string;
	password: string;
};

type AuthState = {
	user?: auth_UserInfo;
	accessToken?: string;
	refreshToken?: string;
};

type AuthContextValue = {
	isAuthenticated: boolean;
	isLoading: boolean;
	error?: string;
	user?: auth_UserInfo;
	accessToken?: string;
	refreshToken?: string;
	login: (credentials: AuthCredentials) => Promise<void>;
	logout: () => void;
};

const STORAGE_KEY = 'afch.auth';

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

const readPersistedState = (): AuthState => {
	if (typeof window === 'undefined') {
		return {};
	}
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) {
			return {};
		}
		const parsed = JSON.parse(raw) as AuthState;
		return parsed ?? {};
	} catch (error) {
		console.warn('读取认证缓存失败', error);
		return {};
	}
};

const writePersistedState = (state: AuthState) => {
	if (typeof window === 'undefined') {
		return;
	}
	if (!state.accessToken) {
		localStorage.removeItem(STORAGE_KEY);
		return;
	}
	localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
};

const applyTokenToClient = (token?: string) => {
	OpenAPI.TOKEN = token;
};

const persistResponse = (
	result: auth_LoginResponse,
	setState: React.Dispatch<React.SetStateAction<AuthState>>,
) => {
	const nextState: AuthState = {
		user: result.user,
		accessToken: result.access_token,
		refreshToken: result.refresh_token,
	};
	setState(nextState);
};

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
	const [state, setState] = useState<AuthState>(() => {
		const initial = readPersistedState();
		applyTokenToClient(initial.accessToken);
		return initial;
	});
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string>();

	useEffect(() => {
		applyTokenToClient(state.accessToken);
		if (state.accessToken) {
			writePersistedState(state);
		} else {
			localStorage.removeItem(STORAGE_KEY);
		}
	}, [state]);

	const handleLogin = useCallback(async (credentials: AuthCredentials) => {
		setIsLoading(true);
		setError(undefined);
		try {
			const result = await AuthService.postApiAuthLogin(credentials);
			persistResponse(result, setState);
			message.success('登录成功');
		} catch (err) {
			const description = err instanceof Error ? err.message : '无法登录，请稍后再试';
			setError(description);
			message.error(description);
			throw err;
		} finally {
			setIsLoading(false);
		}
	}, []);

	const logout = useCallback(() => {
		setState({});
		setError(undefined);
		message.info('已退出登录');
	}, []);

	const contextValue: AuthContextValue = useMemo(() => ({
		isAuthenticated: Boolean(state.accessToken && state.user),
		isLoading,
		error,
		user: state.user,
		accessToken: state.accessToken,
		refreshToken: state.refreshToken,
		login: handleLogin,
		logout,
	}), [state, isLoading, error, handleLogin, logout]);

	return (
		<AuthContext.Provider value={contextValue}>
			{children}
		</AuthContext.Provider>
	);
};

// eslint-disable-next-line react-refresh/only-export-components -- Hook 与 Provider 放在同一文件便于复用上下文类型
export const useAuth = (): AuthContextValue => {
	const value = useContext(AuthContext);
	if (!value) {
		throw new Error('useAuth 必须在 AuthProvider 中使用');
	}
	return value;
};
