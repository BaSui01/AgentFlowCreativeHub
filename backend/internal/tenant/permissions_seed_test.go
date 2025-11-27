package tenant

import "testing"

func TestGetPermissionCatalog(t *testing.T) {
	catalog := GetPermissionCatalog()
	if catalog.Version == "" {
		t.Fatalf("expected version, got empty")
	}
	if len(catalog.Items) != len(systemPermissionCatalog) {
		t.Fatalf("catalog items mismatch: got %d want %d", len(catalog.Items), len(systemPermissionCatalog))
	}
	if len(catalog.CategoryLabels) == 0 {
		t.Fatalf("expected category labels")
	}
	if len(catalog.CategoryOrder) == 0 {
		t.Fatalf("expected category order")
	}
	for _, item := range catalog.Items {
		if item.Locales == nil || len(item.Locales) == 0 {
			t.Fatalf("permission %s missing locales", item.Code)
		}
		if _, ok := item.Locales["zh-CN"]; !ok {
			t.Fatalf("permission %s missing zh-CN locale", item.Code)
		}
	}
}
