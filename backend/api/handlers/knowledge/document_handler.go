package knowledge

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	response "backend/api/handlers/common"
	auditpkg "backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/models"
	"backend/internal/rag"
	"backend/pkg/types"

	"github.com/gin-gonic/gin"
)

// DocumentHandler 文档处理器
type DocumentHandler struct {
	docService *models.DocumentService
	kbService  *models.KnowledgeBaseService
	ragService *rag.RAGService
}

// NewDocumentHandler 创建文档处理器
func NewDocumentHandler(
	docService *models.DocumentService,
	kbService *models.KnowledgeBaseService,
	ragService *rag.RAGService,
) *DocumentHandler {
	return &DocumentHandler{
		docService: docService,
		kbService:  kbService,
		ragService: ragService,
	}
}

// UploadRequest 上传文档请求（multipart/form-data）
// file: 文件
// title: 标题（可选，默认使用文件名）
// source: 来源（可选）

// Upload 上传文档
// @Summary 上传知识库文档
// @Tags KnowledgeBase
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "知识库 ID"
// @Param file formData file true "文档文件"
// @Param title formData string false "文档标题"
// @Param source formData string false "来源"
// @Success 202 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/knowledge-bases/{id}/documents [post]
func (h *DocumentHandler) Upload(c *gin.Context) {
	kbID := c.Param("id")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
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

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "未找到上传文件: " + err.Error()})
		return
	}
	defer file.Close()

	// 读取文件内容
	_, err = io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "读取文件失败: " + err.Error()})
		return
	}

	// 获取文件信息
	filename := header.Filename
	fileSize := header.Size
	contentType := detectContentType(filename)

	// 获取标题（如果未提供则使用文件名）
	title := c.PostForm("title")
	if title == "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	// 获取来源
	source := c.PostForm("source")
	if source == "" {
		source = filename
	}

	// 创建文档记录
	doc := &models.Document{
		KnowledgeBaseID: kbID,
		TenantID:        userCtx.TenantID,
		Title:           title,
		Content:         "", // 将在处理时填充
		ContentType:     contentType,
		Source:          source,
		SourceType:      "file",
		FileSize:        fileSize,
		Status:          "pending",
		CreatedBy:       userCtx.UserID,
	}

	if err := h.docService.CreateDocument(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "创建文档记录失败: " + err.Error()})
		return
	}

	// 异步处理文档
	go func() {
		ctx := c.Request.Context()
		if err := h.ragService.ProcessDocument(ctx, doc.ID); err != nil {
			// 处理失败已在 ProcessDocument 中记录
		}
	}()

	// 设置审计信息
	auditpkg.SetAuditResourceInfo(c, "document", doc.ID)
	auditpkg.SetAuditMetadata(c, "knowledge_base_id", kbID)
	auditpkg.SetAuditMetadata(c, "filename", filename)

	c.JSON(http.StatusAccepted, response.APIResponse{Success: true, Message: "文档已提交处理", Data: gin.H{"document_id": doc.ID, "status": "pending"}})
}

// ListDocuments 列出文档
// @Summary 列出知识库文档
// @Tags KnowledgeBase
// @Security BearerAuth
// @Produce json
// @Param id path string true "知识库 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.ListResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/knowledge-bases/{id}/documents [get]
func (h *DocumentHandler) ListDocuments(c *gin.Context) {
	kbID := c.Param("id")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
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

	// 分页参数
	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if parsed := parseInt(p); parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed := parseInt(ps); parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	pagination := &types.PaginationRequest{
		Page:     page,
		PageSize: pageSize,
	}

	docs, paginationResp, err := h.docService.ListDocuments(c.Request.Context(), kbID, pagination)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询文档失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.ListResponse{
		Items:      docs,
		Pagination: toPaginationMeta(paginationResp),
	})
}

// GetDocument 获取文档详情
// @Summary 文档详情
// @Tags KnowledgeBase
// @Security BearerAuth
// @Produce json
// @Param id path string true "文档 ID"
// @Success 200 {object} models.Document
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/documents/{id} [get]
func (h *DocumentHandler) GetDocument(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少文档 ID"})
		return
	}

	doc, err := h.docService.GetDocument(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询文档失败: " + err.Error()})
		return
	}

	if doc == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "文档不存在"})
		return
	}

	// 检查权限
	userCtx, _ := auth.GetUserContext(c)
	if doc.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	c.JSON(http.StatusOK, doc)
}

// DeleteDocument 删除文档
// @Summary 删除文档
// @Tags KnowledgeBase
// @Security BearerAuth
// @Produce json
// @Param id path string true "文档 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/documents/{id} [delete]
func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少文档 ID"})
		return
	}

	// 获取文档
	doc, err := h.docService.GetDocument(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询文档失败: " + err.Error()})
		return
	}

	if doc == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "文档不存在"})
		return
	}

	// 检查权限
	userCtx, _ := auth.GetUserContext(c)
	if doc.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	// 删除文档（同时删除所有分块）
	if err := h.docService.DeleteDocument(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "删除文档失败: " + err.Error()})
		return
	}

	// 更新知识库统计
	go h.kbService.UpdateStats(c.Request.Context(), doc.KnowledgeBaseID)

	// 设置审计信息
	auditpkg.SetAuditResourceInfo(c, "document", id)

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "删除成功"})
}

// ListChunks 列出文档分块
// @Summary 文档分块列表
// @Tags KnowledgeBase
// @Security BearerAuth
// @Produce json
// @Param id path string true "文档 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/documents/{id}/chunks [get]
func (h *DocumentHandler) ListChunks(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少文档 ID"})
		return
	}

	// 获取文档
	doc, err := h.docService.GetDocument(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询文档失败: " + err.Error()})
		return
	}

	if doc == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: "文档不存在"})
		return
	}

	// 检查权限
	userCtx, _ := auth.GetUserContext(c)
	if doc.TenantID != userCtx.TenantID {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Success: false, Message: "无权访问"})
		return
	}

	// 获取分块列表
	chunks, err := h.docService.ListChunks(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "查询分块失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{
		"document_id": id,
		"chunks":      chunks,
		"total":       len(chunks),
	}})
}

// detectContentType 根据文件扩展名检测内容类型
func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	contentTypes := map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".html": "text/html",
		".htm":  "text/html",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}

	if ct, exists := contentTypes[ext]; exists {
		return ct
	}

	return "text/plain" // 默认
}

// CreateTextDocumentRequest 创建文本文档请求
type CreateTextDocumentRequest struct {
	Title       string `json:"title" binding:"required,min=1,max=500"`
	Content     string `json:"content" binding:"required,min=1"`
	ContentType string `json:"content_type"`
}

// CreateTextDocument 创建文本文档（直接输入文本）
// @Summary 创建知识库文本文档
// @Tags KnowledgeBase
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "知识库 ID"
// @Param request body CreateTextDocumentRequest true "文档内容"
// @Success 202 {object} response.APIResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/knowledge-bases/{id}/documents/text [post]
func (h *DocumentHandler) CreateTextDocument(c *gin.Context) {
	kbID := c.Param("id")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少知识库 ID"})
		return
	}

	var req CreateTextDocumentRequest
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

	// 默认内容类型
	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}

	// 创建文档记录
	doc := &models.Document{
		KnowledgeBaseID: kbID,
		TenantID:        userCtx.TenantID,
		Title:           req.Title,
		Content:         "", // 将在处理时填充
		ContentType:     contentType,
		Source:          "manual",
		SourceType:      "manual",
		FileSize:        int64(len(req.Content)),
		Status:          "pending",
		CreatedBy:       userCtx.UserID,
	}

	if err := h.docService.CreateDocument(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: "创建文档记录失败: " + err.Error()})
		return
	}

	// 异步处理文档
	go func() {
		ctx := c.Request.Context()
		if err := h.ragService.ProcessDocument(ctx, doc.ID); err != nil {
			fmt.Printf("处理文档失败: %v\n", err)
		}
	}()

	// 设置审计信息
	auditpkg.SetAuditResourceInfo(c, "document", doc.ID)
	auditpkg.SetAuditMetadata(c, "knowledge_base_id", kbID)

	c.JSON(http.StatusAccepted, response.APIResponse{Success: true, Message: "文档已提交处理", Data: gin.H{"document_id": doc.ID, "status": "pending"}})
}
