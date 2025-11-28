package audit

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"backend/internal/tenant"
)

// ExportFormat 导出格式
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
)

// ExportRequest 导出请求
type ExportRequest struct {
	TenantID  string       `json:"tenantId"`
	Format    ExportFormat `json:"format"`
	From      *time.Time   `json:"from,omitempty"`
	To        *time.Time   `json:"to,omitempty"`
	UserID    string       `json:"userId,omitempty"`
	Action    string       `json:"action,omitempty"`
	Limit     int          `json:"limit,omitempty"` // 最大导出条数
}

// ExportResult 导出结果
type ExportResult struct {
	Data        []byte `json:"data,omitempty"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	TotalCount  int    `json:"totalCount"`
}

// AuditLogExporter 审计日志导出器
type AuditLogExporter struct {
	reader AuditLogReader
}

// NewAuditLogExporter 创建导出器
func NewAuditLogExporter(reader AuditLogReader) *AuditLogExporter {
	return &AuditLogExporter{reader: reader}
}

// Export 导出审计日志
func (e *AuditLogExporter) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	tc := tenant.TenantContext{TenantID: req.TenantID}
	
	// 设置默认和最大限制
	limit := req.Limit
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
	
	filter := AuditLogFilter{
		From:   req.From,
		To:     req.To,
		UserID: req.UserID,
		Action: req.Action,
		Limit:  limit,
		Offset: 0,
	}
	
	logs, err := e.reader.QueryTenantLogs(ctx, tc, filter)
	if err != nil {
		return nil, fmt.Errorf("查询审计日志失败: %w", err)
	}
	
	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	
	switch req.Format {
	case FormatCSV:
		return e.exportCSV(logs, timestamp)
	case FormatJSON:
		return e.exportJSON(logs, timestamp)
	default:
		return e.exportJSON(logs, timestamp)
	}
}

// exportCSV 导出为 CSV 格式
func (e *AuditLogExporter) exportCSV(logs []tenant.AuditLog, timestamp string) (*ExportResult, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	
	// 写入表头
	header := []string{"ID", "租户ID", "用户ID", "操作", "资源", "详情", "创建时间"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	
	// 写入数据
	for _, log := range logs {
		detailsStr := ""
		if log.Details != nil {
			if b, err := json.Marshal(log.Details); err == nil {
				detailsStr = string(b)
			}
		}
		
		row := []string{
			log.ID,
			log.TenantID,
			log.UserID,
			log.Action,
			log.Resource,
			detailsStr,
			log.CreatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	
	return &ExportResult{
		Data:        buf.Bytes(),
		Filename:    fmt.Sprintf("audit_logs_%s.csv", timestamp),
		ContentType: "text/csv; charset=utf-8",
		TotalCount:  len(logs),
	}, nil
}

// exportJSON 导出为 JSON 格式
func (e *AuditLogExporter) exportJSON(logs []tenant.AuditLog, timestamp string) (*ExportResult, error) {
	// 转换为导出格式
	exportLogs := make([]AuditLogExport, len(logs))
	for i, log := range logs {
		exportLogs[i] = AuditLogExport{
			ID:        log.ID,
			TenantID:  log.TenantID,
			UserID:    log.UserID,
			Action:    log.Action,
			Resource:  log.Resource,
			Details:   log.Details,
			CreatedAt: log.CreatedAt.Format(time.RFC3339),
		}
	}
	
	result := AuditLogExportResult{
		ExportedAt: time.Now().Format(time.RFC3339),
		TotalCount: len(logs),
		Logs:       exportLogs,
	}
	
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	
	return &ExportResult{
		Data:        data,
		Filename:    fmt.Sprintf("audit_logs_%s.json", timestamp),
		ContentType: "application/json; charset=utf-8",
		TotalCount:  len(logs),
	}, nil
}

// ExportToWriter 导出到 io.Writer（用于流式导出）
func (e *AuditLogExporter) ExportToWriter(ctx context.Context, req *ExportRequest, w io.Writer) error {
	result, err := e.Export(ctx, req)
	if err != nil {
		return err
	}
	_, err = w.Write(result.Data)
	return err
}

// AuditLogExport 导出格式的审计日志
type AuditLogExport struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenantId"`
	UserID    string `json:"userId,omitempty"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	Details   any    `json:"details,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// AuditLogExportResult JSON 导出结果包装
type AuditLogExportResult struct {
	ExportedAt string           `json:"exportedAt"`
	TotalCount int              `json:"totalCount"`
	Logs       []AuditLogExport `json:"logs"`
}
