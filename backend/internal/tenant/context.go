package tenant

import "context"

// TenantContext carries tenant and user identity information through the request lifecycle.
// It is intended to be populated once at the HTTP boundary and then passed down into
// services and repositories that require tenant-aware behavior.
type TenantContext struct {
	TenantID      string
	UserID        string
	Roles         []string
	IsSystemAdmin bool
}

type tenantContextKey struct{}

// WithTenantContext attaches the given TenantContext to the provided context and returns
// a derived context. Callers should use this helper instead of storing TenantContext
// under arbitrary keys.
func WithTenantContext(ctx context.Context, tc TenantContext) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tc)
}

// FromContext attempts to retrieve a TenantContext from the given context. The second
// return value indicates whether a TenantContext was present.
func FromContext(ctx context.Context) (TenantContext, bool) {
	value := ctx.Value(tenantContextKey{})
	if value == nil {
		return TenantContext{}, false
	}

	tc, ok := value.(TenantContext)
	if !ok {
		return TenantContext{}, false
	}

	return tc, true
}

// MustTenantContext retrieves the TenantContext from the given context and panics if it
// is missing. It is suitable for places where the presence of a tenant has been guaranteed
// by earlier middleware and its absence indicates a programming error.
func MustTenantContext(ctx context.Context) TenantContext {
	tc, ok := FromContext(ctx)
	if !ok {
		panic("tenant: TenantContext missing from context")
	}

	return tc
}
