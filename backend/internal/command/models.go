package command

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CommandRequest 记录命令执行元数据
type CommandRequest struct {
	ID              string         `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID        string         `gorm:"type:uuid;index" json:"tenantId"`
	CommandType     string         `gorm:"size:64" json:"commandType"`
	AgentID         string         `gorm:"type:uuid;index" json:"agentId"`
	SessionID       string         `gorm:"size:255" json:"sessionId"`
	Status          string         `gorm:"size:32;index" json:"status"`
	ContextRevision string         `gorm:"size:64" json:"contextRevisionId"`
	ContextSnapshot string         `gorm:"type:text" json:"contextSnapshot"`
	ContextNodeIDs  datatypes.JSON `json:"contextNodeIds"`
	ContextDigest   string         `gorm:"size:64" json:"contextDigest"`
	Notes           string         `gorm:"type:text" json:"notes"`
	Priority        int            `json:"priority"`
	QueueKey        string         `gorm:"size:128" json:"queueKey"`
	QueuePosition   int            `json:"queuePosition"`
	DedupKey        string         `gorm:"size:64;index" json:"dedupKey"`
	DeadlineAt      *time.Time     `json:"deadlineAt"`
	ResultPreview   string         `gorm:"type:text" json:"resultPreview"`
	FailureReason   string         `gorm:"type:text" json:"failureReason"`
	TokenCost       int            `json:"tokenCost"`
	LatencyMs       int            `json:"latencyMs"`
	TraceID         string         `gorm:"size:64" json:"traceId"`
	CreatedBy       string         `gorm:"type:uuid" json:"createdBy"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

func (r *CommandRequest) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	if r.UpdatedAt.IsZero() {
		r.UpdatedAt = r.CreatedAt
	}
	return nil
}

func (r *CommandRequest) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now().UTC()
	return nil
}
