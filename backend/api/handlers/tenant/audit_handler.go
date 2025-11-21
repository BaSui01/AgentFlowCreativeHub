package tenant

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/internal/audit"
	"backend/internal/tenant"
)

// AuditHandler 负责处理当前租户的审计日志查询请求。
//
// 约定：
//   GET /api/tenant/audit-logs?from=ISO8601&to=ISO8601&userId=...&action=...&limit=...&offset=...
//
// 实际路由挂载将在 router 中完成（Task 10）。
type AuditHandler struct {
	reader audit.AuditLogReader
}

func NewAuditHandler(reader audit.AuditLogReader) *AuditHandler {
	return &AuditHandler{reader: reader}
}

// GetTenantAuditLogs 返回当前 TenantContext 中租户的审计日志列表。
func (h *AuditHandler) GetTenantAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tc, ok := tenant.FromContext(ctx)
	if !ok || tc.TenantID == "" {
		writeError(w, http.StatusForbidden, "missing tenant context")
		return
	}

	q := r.URL.Query()

	var fromPtr, toPtr *time.Time
	if fromStr := q.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			fromPtr = &t
		}
	}
	if toStr := q.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			toPtr = &t
		}
	}

	limit := parseIntOrDefault(q.Get("limit"), 50)
	offset := parseIntOrDefault(q.Get("offset"), 0)

	filter := audit.AuditLogFilter{
		From:   fromPtr,
		To:     toPtr,
		UserID: q.Get("userId"),
		Action: q.Get("action"),
		Limit:  limit,
		Offset: offset,
	}

	logs, err := h.reader.QueryTenantLogs(ctx, tc, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query audit logs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": logs,
	})
}

func parseIntOrDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": msg,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
