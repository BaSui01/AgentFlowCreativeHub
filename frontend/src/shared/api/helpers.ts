export const extractPayload = <T>(payload: { data?: T } | T): T => {
	if (payload && typeof payload === 'object' && 'data' in (payload as Record<string, unknown>)) {
		const nested = (payload as { data?: T }).data;
		return nested ?? (payload as T);
	}
	return payload as T;
};
