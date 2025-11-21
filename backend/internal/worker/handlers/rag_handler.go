package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"backend/internal/rag"
	"backend/internal/worker/tasks"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

type RAGHandler struct {
	ragService *rag.RAGService
	logger     *zap.Logger
}

func NewRAGHandler(ragService *rag.RAGService, logger *zap.Logger) *RAGHandler {
	return &RAGHandler{
		ragService: ragService,
		logger:     logger,
	}
}

func (h *RAGHandler) HandleProcessDocument(ctx context.Context, t *asynq.Task) error {
	var p tasks.ProcessDocumentPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json unmarshal failed: %w", err)
	}

	h.logger.Info("开始处理文档任务", zap.String("document_id", p.DocumentID))

	if err := h.ragService.ProcessDocument(ctx, p.DocumentID); err != nil {
		h.logger.Error("文档处理失败", zap.String("document_id", p.DocumentID), zap.Error(err))
		return err
	}

	h.logger.Info("文档处理完成", zap.String("document_id", p.DocumentID))
	return nil
}
