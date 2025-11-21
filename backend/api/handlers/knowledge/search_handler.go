package knowledge

import (
	"net/http"

	response "backend/api/handlers/common"
	auditpkg "backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/models"
	"backend/internal/rag"

	"github.com/gin-gonic/gin"
)

// SearchHandler 检索处理器
type SearchHandler struct {
	kbService  *models.KnowledgeBaseService
	ragService *rag.RAGService
}

// NewSearchHandler 创建检索处理器
func NewSearchHandler(
	kbService *models.KnowledgeBaseService,
	ragService *rag.RAGService,
) *SearchHandler {
	return &SearchHandler{
		kbService:  kbService,
		ragService: ragService,
	}
}

// SearchRequest 检索请求
type SearchRequest struct {
	Query string `json:"query" binding:"required,min=1"`
	TopK  int    `json:"top_k"`
}

// Search 语义检索
// @Summary 语义检索
// @Tags KnowledgeBase
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "知识库 ID"
// @Param request body SearchRequest true "检索请求"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/knowledge-bases/{id}/search [post]
func (h *SearchHandler) Search(c *gin.Context) {
	kbID := c.Param("id")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	// 获取用户上下文
	userCtx, _ := auth.GetUserContext(c)

	// 验证知识库存在且有权限
	kb, err := h.kbService.GetKnowledgeBase(c.Request.Context(), kbID)
	if err != nil || kb == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "知识库不存在"})
		return
	}
	if kb.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	// 默认 TopK
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}

	// 执行检索
	searchReq := &rag.SearchRequest{
		KnowledgeBaseID: kbID,
		TenantID:        userCtx.TenantID,
		Query:           req.Query,
		TopK:            topK,
	}

	results, err := h.ragService.Search(c.Request.Context(), searchReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "检索失败: " + err.Error()})
		return
	}

	// 设置审计信息
	auditpkg.SetAuditResourceInfo(c, "knowledge_base", kbID)
	auditpkg.SetAuditMetadata(c, "query", req.Query)
	auditpkg.SetAuditMetadata(c, "result_count", len(results.Results))

	c.JSON(http.StatusOK, gin.H{
		"knowledge_base_id": kbID,
		"query":             req.Query,
		"results":           results.Results,
		"total":             len(results.Results),
	})
}

// GetContextRequest 获取上下文请求
type GetContextRequest struct {
	Query     string `json:"query" binding:"required,min=1"`
	MaxChunks int    `json:"max_chunks"`
}

// GetContext 获取查询上下文（为 Agent 使用）
// @Summary 获取检索上下文
// @Tags KnowledgeBase
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "知识库 ID"
// @Param request body GetContextRequest true "上下文请求"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/knowledge-bases/{id}/context [post]
func (h *SearchHandler) GetContext(c *gin.Context) {
	kbID := c.Param("id")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}

	var req GetContextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}

	// 获取用户上下文
	userCtx, _ := auth.GetUserContext(c)

	// 验证知识库存在且有权限
	kb, err := h.kbService.GetKnowledgeBase(c.Request.Context(), kbID)
	if err != nil || kb == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "知识库不存在"})
		return
	}
	if kb.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	// 默认 MaxChunks
	maxChunks := req.MaxChunks
	if maxChunks <= 0 {
		maxChunks = 3
	}
	if maxChunks > 10 {
		maxChunks = 10
	}

	// 获取上下文（使用 Search 方法）
	searchReq := &rag.SearchRequest{
		KnowledgeBaseID: kbID,
		TenantID:        userCtx.TenantID,
		Query:           req.Query,
		TopK:            maxChunks,
	}

	searchResults, err := h.ragService.Search(c.Request.Context(), searchReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "获取上下文失败: " + err.Error()})
		return
	}

	// 将搜索结果转换为上下文文本
	var contextParts []string
	for _, result := range searchResults.Results {
		contextParts = append(contextParts, result.Content)
	}
	context := ""
	if len(contextParts) > 0 {
		context = contextParts[0] // 简化实现：返回第一个结果
		for i := 1; i < len(contextParts); i++ {
			context += "\n\n" + contextParts[i]
		}
	}

	// 设置审计信息
	auditpkg.SetAuditResourceInfo(c, "knowledge_base", kbID)
	auditpkg.SetAuditMetadata(c, "query", req.Query)

	c.JSON(http.StatusOK, gin.H{
		"knowledge_base_id": kbID,
		"query":             req.Query,
		"context":           context,
	})
}
