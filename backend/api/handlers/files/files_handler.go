package files

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	response "backend/api/handlers/common"
	workspaceSvc "backend/internal/workspace"

	"github.com/gin-gonic/gin"
)

// Handler 文件服务接口
type Handler struct {
	svc *workspaceSvc.Service
}

// NewHandler 构造函数
func NewHandler(svc *workspaceSvc.Service) *Handler {
	return &Handler{svc: svc}
}

// GetTree 返回文件树
func (h *Handler) GetTree(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	_ = h.svc.EnsureDefaults(c.Request.Context(), tenantID, userID)
	depth := 2
	if raw := strings.TrimSpace(c.Query("depth")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			depth = parsed
		}
	}
	var cursor *string
	if v := strings.TrimSpace(c.Query("cursor")); v != "" {
		cursor = &v
	}
	data, err := h.svc.ListTreeWithOptions(c.Request.Context(), tenantID, workspaceSvc.TreeListOptions{
		ParentID: cursor,
		Depth:    depth,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"nodes": data}})
}

// GetContent 获取文件内容
func (h *Handler) GetContent(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	nodeID := c.Query("nodeId")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少 nodeId"})
		return
	}
	file, err := h.svc.GetFileDetail(c.Request.Context(), tenantID, nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: file})
}

type saveFileDTO struct {
	NodeID   string  `json:"nodeId"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
	Category string  `json:"category"`
	Content  string  `json:"content" binding:"required"`
	Summary  string  `json:"summary"`
	AgentID  string  `json:"agentId"`
	ToolName string  `json:"toolName"`
	Metadata string  `json:"metadata"`
}

// CreateFile 新建或保存文件
func (h *Handler) CreateFile(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	var dto saveFileDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	if strings.TrimSpace(dto.NodeID) != "" {
		expected := strings.TrimSpace(c.GetHeader("If-Match"))
		if expected == "" {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Code: "AFCH-FILE-409", Message: "缺少 If-Match 头以进行版本校验"})
			return
		}
		file, err := h.svc.UpdateFileContent(c.Request.Context(), &workspaceSvc.UpdateFileRequest{
			TenantID:          tenantID,
			NodeID:            dto.NodeID,
			Content:           dto.Content,
			Summary:           dto.Summary,
			AgentID:           dto.AgentID,
			ToolName:          dto.ToolName,
			Metadata:          dto.Metadata,
			UserID:            userID,
			ExpectedVersionID: expected,
		})
		if err != nil {
			if errors.Is(err, workspaceSvc.ErrFileVersionConflict) {
				c.JSON(http.StatusConflict, response.ErrorResponse{Success: false, Code: "AFCH-FILE-409", Message: err.Error()})
				return
			}
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: file})
		return
	}
	if strings.TrimSpace(dto.Name) == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "文件名不能为空"})
		return
	}
	file, err := h.svc.CreateFile(c.Request.Context(), &workspaceSvc.CreateFileRequest{
		TenantID: tenantID,
		ParentID: dto.ParentID,
		Name:     dto.Name,
		Category: dto.Category,
		Content:  dto.Content,
		Summary:  dto.Summary,
		AgentID:  dto.AgentID,
		ToolName: dto.ToolName,
		Metadata: dto.Metadata,
		UserID:   userID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response.APIResponse{Success: true, Data: file})
}

type patchFileDTO struct {
	NodeID   string  `json:"nodeId" binding:"required"`
	Name     *string `json:"name"`
	ParentID *string `json:"parentId"`
}

// PatchFile 重命名/移动文件
func (h *Handler) PatchFile(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	var dto patchFileDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	var expected *time.Time
	if header := strings.TrimSpace(c.GetHeader("If-Match")); header != "" {
		if ts, err := time.Parse(time.RFC3339Nano, header); err == nil {
			expected = &ts
		}
	}
	node, err := h.svc.PatchFile(c.Request.Context(), &workspaceSvc.PatchFileRequest{
		TenantID:          tenantID,
		NodeID:            dto.NodeID,
		NewName:           dto.Name,
		NewParentID:       dto.ParentID,
		UserID:            userID,
		ExpectedUpdatedAt: expected,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: node})
}

// DeleteFile 删除文件
func (h *Handler) DeleteFile(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少文件ID"})
		return
	}
	if err := h.svc.DeleteNode(c.Request.Context(), tenantID, nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "删除成功"})
}

type searchFileDTO struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// SearchFiles 搜索
func (h *Handler) SearchFiles(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	var dto searchFileDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	items, total, err := h.svc.SearchFiles(c.Request.Context(), &workspaceSvc.SearchFilesRequest{
		TenantID: tenantID,
		Query:    dto.Query,
		Limit:    dto.Limit,
		Offset:   dto.Offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{
		"items":     items,
		"totalHits": total,
		"limit":     dto.Limit,
		"offset":    dto.Offset,
	}})
}

// GetHistory 版本记录
func (h *Handler) GetHistory(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	nodeID := c.Param("nodeId")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少 nodeId"})
		return
	}
	limit := 20
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}
	items, err := h.svc.GetFileHistory(c.Request.Context(), tenantID, nodeID, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{"items": items}})
}

type revertFileDTO struct {
	NodeID    string `json:"nodeId" binding:"required"`
	VersionID string `json:"versionId" binding:"required"`
}

// RevertFile 恢复版本
func (h *Handler) RevertFile(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	var dto revertFileDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	file, err := h.svc.RevertFile(c.Request.Context(), tenantID, dto.NodeID, dto.VersionID, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: file})
}

// DiffVersions 差异比对
func (h *Handler) DiffVersions(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	versionA := c.Query("versionA")
	versionB := c.Query("versionB")
	if versionA == "" || versionB == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少版本ID"})
		return
	}
	result, err := h.svc.DiffFileVersions(c.Request.Context(), tenantID, versionA, versionB)
	if err != nil {
		if errors.Is(err, workspaceSvc.ErrDiffVersionNotFound) {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Code: "AFCH-DIFF-404", Message: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: result})
}

// ============================================
// 文件上传下载 API
// ============================================

type initiateUploadDTO struct {
	FileName  string  `json:"fileName" binding:"required"`
	MimeType  string  `json:"mimeType"`
	FileSize  int64   `json:"fileSize" binding:"required"`
	ChunkSize int64   `json:"chunkSize"`
	ParentID  *string `json:"parentId"`
}

// InitiateUpload 初始化分片上传
func (h *Handler) InitiateUpload(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	var dto initiateUploadDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "参数错误: " + err.Error()})
		return
	}
	result, err := h.svc.InitiateUpload(c.Request.Context(), &workspaceSvc.InitiateUploadRequest{
		TenantID:  tenantID,
		FileName:  dto.FileName,
		MimeType:  dto.MimeType,
		FileSize:  dto.FileSize,
		ChunkSize: dto.ChunkSize,
		ParentID:  dto.ParentID,
		UserID:    userID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: result})
}

// UploadChunk 上传分片
func (h *Handler) UploadChunk(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	uploadID := c.Param("uploadId")
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少上传ID"})
		return
	}

	chunkIndexStr := c.PostForm("chunkIndex")
	if chunkIndexStr == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少分片索引"})
		return
	}
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "分片索引无效"})
		return
	}

	file, _, err := c.Request.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "获取文件失败: " + err.Error()})
		return
	}
	defer file.Close()

	if err := h.svc.UploadChunk(c.Request.Context(), &workspaceSvc.UploadChunkRequest{
		TenantID:   tenantID,
		UploadID:   uploadID,
		ChunkIndex: chunkIndex,
		ChunkData:  file,
		UserID:     userID,
	}); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "分片上传成功"})
}

type completeUploadDTO struct {
	ParentID *string `json:"parentId"`
}

// CompleteUpload 完成上传
func (h *Handler) CompleteUpload(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	uploadID := c.Param("uploadId")
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少上传ID"})
		return
	}

	var dto completeUploadDTO
	_ = c.ShouldBindJSON(&dto)

	result, err := h.svc.CompleteUpload(c.Request.Context(), tenantID, uploadID, userID, dto.ParentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: result})
}

// UploadFile 单文件上传（小文件）
func (h *Handler) UploadFile(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "获取文件失败: " + err.Error()})
		return
	}
	defer file.Close()

	parentID := c.PostForm("parentId")
	var parentIDPtr *string
	if parentID != "" {
		parentIDPtr = &parentID
	}

	result, err := h.svc.UploadSingleFile(c.Request.Context(), &workspaceSvc.UploadFileRequest{
		TenantID: tenantID,
		FileName: header.Filename,
		MimeType: header.Header.Get("Content-Type"),
		FileSize: header.Size,
		ParentID: parentIDPtr,
		UserID:   userID,
	}, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response.APIResponse{Success: true, Data: result})
}

// DownloadFile 下载文件
func (h *Handler) DownloadFile(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	nodeID := c.Param("nodeId")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少文件ID"})
		return
	}

	upload, err := h.svc.GetFileForDownload(c.Request.Context(), tenantID, nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\""+upload.OriginalName+"\"")
	c.Header("Content-Type", upload.MimeType)
	c.File(upload.StoragePath)
}

// GetPreview 获取文件预览
func (h *Handler) GetPreview(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	nodeID := c.Param("nodeId")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少文件ID"})
		return
	}

	preview, err := h.svc.GetFilePreview(c.Request.Context(), tenantID, nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: preview})
}

// ListUploads 列出上传记录
func (h *Handler) ListUploads(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	limit := 20
	offset := 0
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}
	if v := c.Query("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			offset = parsed
		}
	}

	uploads, total, err := h.svc.ListUploads(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Data: gin.H{
		"items": uploads,
		"total": total,
		"limit": limit,
		"offset": offset,
	}})
}

// DeleteUpload 删除上传文件
func (h *Handler) DeleteUpload(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	uploadID := c.Param("uploadId")
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: "缺少上传ID"})
		return
	}

	if err := h.svc.DeleteUpload(c.Request.Context(), tenantID, uploadID); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse{Success: true, Message: "删除成功"})
}
