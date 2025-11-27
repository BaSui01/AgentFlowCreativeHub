import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { message } from 'antd';

import type { AuthLoginRequest, AuthLoginResponse, AuthTokenPair, AuthUserInfo } from '@/api/generated/model';
import { AXIOS_INSTANCE } from '@/api/instance';
import { AuthAPI } from '@/shared/api/auth';

type AuthCredentials = AuthLoginRequest;

type AuthState = {
	user?: AuthUserInfo;
	accessToken?: string;
	refreshToken?: string;
};

type AuthContextValue = {
	isAuthenticated: boolean;
	isLoading: boolean;
	error?: string;
	user?: AuthUserInfo;
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

const extractStateFromLogin = (payload?: AuthLoginResponse): AuthState => ({
	user: payload?.user,
	accessToken: payload?.access_token,
	refreshToken: payload?.refresh_token,
});

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
	const [state, setState] = useState<AuthState>(() => readPersistedState());
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string>();
	const refreshPromiseRef = useRef<Promise<string | undefined> | null>(null);
	const refreshTokenRef = useRef<string | undefined>(state.refreshToken);

	useEffect(() => {
		refreshTokenRef.current = state.refreshToken;
	}, [state.refreshToken]);

	useEffect(() => {
		if (state.accessToken) {
			writePersistedState(state);
		} else {
			localStorage.removeItem(STORAGE_KEY);
		}
	}, [state]);

	const refreshTokens = useCallback(async (): Promise<string | undefined> => {
		const refreshToken = refreshTokenRef.current;
		if (!refreshToken) {
			throw new Error('缺少刷新令牌');
		}
		const tokens: AuthTokenPair = await AuthAPI.refresh({ refresh_token: refreshToken });
		return new Promise<string | undefined>((resolve) => {
			setState((prev) => {
				const nextState: AuthState = {
					user: prev.user,
					accessToken: tokens.access_token ?? prev.accessToken,
					refreshToken: tokens.refresh_token ?? prev.refreshToken,
				};
				resolve(nextState.accessToken);
				return nextState;
			});
		});
	}, []);

	const handleLogin = useCallback(async (credentials: AuthCredentials) => {
		setIsLoading(true);
		setError(undefined);
		try {
			const result = await AuthAPI.login(credentials);
			setState(extractStateFromLogin(result));
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
		const refreshToken = refreshTokenRef.current;
		refreshPromiseRef.current = null;
		setState({});
		setError(undefined);
		if (refreshToken) {
			void AuthAPI.logout({ refresh_token: refreshToken }).catch(() => undefined);
		}
		message.info('已退出登录');
	}, []);

	useEffect(() => {
		const requestId = AXIOS_INSTANCE.interceptors.request.use((config) => {
			if (state.accessToken) {
				config.headers = {
					...config.headers,
					Authorization: `Bearer ${state.accessToken}`,
				};
			}
			return config;
		});

		const responseId = AXIOS_INSTANCE.interceptors.response.use(
			(response) => response,
			async (error) => {
				const { response, config } = error ?? {};
				if (!response || response.status !== 401 || !config) {
					return Promise.reject(error);
				}
				if ((config as Record<string, unknown>).__authRetry || !refreshTokenRef.current) {
					logout();
					return Promise.reject(error);
				}
				if (!refreshPromiseRef.current) {
					refreshPromiseRef.current = refreshTokens()
						.catch((refreshError) => {
							refreshPromiseRef.current = null;
							throw refreshError;
						})
						.finally(() => {
							refreshPromiseRef.current = null;
						});
				}
				try {
					const newToken = await refreshPromiseRef.current;
					if (!newToken) {
						logout();
						return Promise.reject(error);
					}
					(config as Record<string, unknown>).__authRetry = true;
					config.headers = {
						...config.headers,
						Authorization: `Bearer ${newToken}`,
					};
					return AXIOS_INSTANCE(config);
				} catch (refreshError) {
					logout();
					return Promise.reject(refreshError);
				}
			},
		);

		return () => {
			AXIOS_INSTANCE.interceptors.request.eject(requestId);
			AXIOS_INSTANCE.interceptors.response.eject(responseId);
		};
	}, [state.accessToken, refreshTokens, logout]);

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
