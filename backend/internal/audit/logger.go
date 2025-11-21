package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"backend/internal/infra"
	"backend/internal/tenant"
)

// DBAuditLogger 实现 tenant.AuditLogger，将审计事件写入 audit_logs 表。
//
// 该实现依赖 TenantContext 中的 tenantId/userId 来保证多租户隔离，
// 不会在接口层返回错误，写入失败时静默忽略（可以根据项目日志系统扩展）。
type DBAuditLogger struct {
	db  infra.DB
	ids tenant.IDGenerator
}

// NewDBAuditLogger 创建一个基于 DB 的审计日志记录器。
func NewDBAuditLogger(db infra.DB, ids tenant.IDGenerator) *DBAuditLogger {
	return &DBAuditLogger{db: db, ids: ids}
}

// LogAction 实现 tenant.AuditLogger 接口，将操作写入 audit_logs 表。
func (l *DBAuditLogger) LogAction(ctx context.Context, tc tenant.TenantContext, action, resource string, details any) {
	if ctx == nil || tc.TenantID == "" {
		return
	}

	id, err := l.ids.NewID()
	if err != nil {
		return
	}

	var detailsJSON []byte
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailsJSON = b
		}
	}

	const q = `
		INSERT INTO audit_logs (id, tenant_id, user_id, action, resource, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	// 写入失败时不向上抛错，避免业务流程因审计失败而中断。
	_, _ = l.db.ExecContext(ctx, q,
		id,
		tc.TenantID,
		nullableString(tc.UserID),
		action,
		resource,
		jsonOrNull(detailsJSON),
		time.Now().UTC(),
	)
}

// SecurityEventLogger 用于记录安全相关事件（如跨租户访问尝试等）。
type SecurityEventLogger interface {
	LogSecurityEvent(ctx context.Context, tc tenant.TenantContext, category string, details any)
}

// DBSecurityEventLogger 复用 DBAuditLogger，将安全事件统一写入 audit_logs，
// action 按 "security.<category>" 规范化，resource 固定为 "security_event"。
type DBSecurityEventLogger struct {
	audit *DBAuditLogger
}

func NewDBSecurityEventLogger(audit *DBAuditLogger) *DBSecurityEventLogger {
	return &DBSecurityEventLogger{audit: audit}
}

func (l *DBSecurityEventLogger) LogSecurityEvent(ctx context.Context, tc tenant.TenantContext, category string, details any) {
	if category == "" {
		category = "generic"
	}
	action := "security." + category
	l.audit.LogAction(ctx, tc, action, "security_event", details)
}

// AuditLogFilter 封装审计日志查询条件。
type AuditLogFilter struct {
	From     *time.Time
	To       *time.Time
	UserID   string
	Action   string
	Limit    int
	Offset   int
}

// AuditLogReader 定义按租户查询审计日志的接口，供 HTTP Handler 使用。
type AuditLogReader interface {
	QueryTenantLogs(ctx context.Context, tc tenant.TenantContext, f AuditLogFilter) ([]tenant.AuditLog, error)
}

// QueryTenantLogs 在 DBAuditLogger 上实现 AuditLogReader，按 tenant_id 及筛选条件查询审计日志。
func (l *DBAuditLogger) QueryTenantLogs(ctx context.Context, tc tenant.TenantContext, f AuditLogFilter) ([]tenant.AuditLog, error) {
	if ctx == nil || tc.TenantID == "" {
		return nil, nil
	}

	// 构建动态 WHERE 条件
	where := "WHERE tenant_id = $1"
	args := []any{tc.TenantID}
	idx := 2

	if f.From != nil {
		where += " AND created_at >= $" + itoa(idx)
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where += " AND created_at <= $" + itoa(idx)
		args = append(args, *f.To)
		idx++
	}
	if f.UserID != "" {
		where += " AND user_id = $" + itoa(idx)
		args = append(args, f.UserID)
		idx++
	}
	if f.Action != "" {
		where += " AND action = $" + itoa(idx)
		args = append(args, f.Action)
		idx++
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := "SELECT id, tenant_id, user_id, action, resource, details, created_at FROM audit_logs " + where + " ORDER BY created_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []tenant.AuditLog
	for rows.Next() {
		var (
			id       string
			tenantID string
			userID   sql.NullString
			action   string
			resource string
			details  json.RawMessage
			created  time.Time
		)
		if err := rows.Scan(&id, &tenantID, &userID, &action, &resource, &details, &created); err != nil {
			return nil, err
		}

		var detailsAny any
		if len(details) > 0 {
			_ = json.Unmarshal(details, &detailsAny)
		}

		logs = append(logs, tenant.AuditLog{
			ID:        id,
			TenantID:  tenantID,
			UserID:    userID.String,
			Action:    action,
			Resource:  resource,
			Details:   detailsAny,
			CreatedAt: created,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}

// nullableString 将空字符串转换为 sql.NullString。
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// jsonOrNull 根据内容决定是否将 JSON 作为 NULL 传入。
func jsonOrNull(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// itoa 是一个最小化的整数转字符串实现，避免引入 fmt 依赖。
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
