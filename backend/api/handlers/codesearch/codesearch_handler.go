package codesearch

import (
	"net/http"

	"backend/api/handlers/common"
	"backend/internal/codesearch"

	"github.com/gin-gonic/gin"
)

// Handler 代码搜索 Handler
type Handler struct {
	aceService      *codesearch.ACECodeSearchService
	codebaseService *codesearch.CodebaseSearchService
}

// NewHandler 创建代码搜索 Handler
func NewHandler(aceService *codesearch.ACECodeSearchService, codebaseService *codesearch.CodebaseSearchService) *Handler {
	return &Handler{
		aceService:      aceService,
		codebaseService: codebaseService,
	}
}

// SymbolSearchRequest 符号搜索请求
type SymbolSearchRequest struct {
	Query      string `json:"query" binding:"required"`
	SymbolType string `json:"symbol_type,omitempty"`
	Language   string `json:"language,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// SearchSymbols 搜索代码符号
// @Summary 搜索代码符号
// @Description 搜索函数、类、接口、方法等代码符号
// @Tags CodeSearch
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body SymbolSearchRequest true "搜索请求"
// @Success 200 {object} common.APIResponse{data=codesearch.SemanticSearchResult}
// @Router /api/codesearch/symbols [post]
func (h *Handler) SearchSymbols(c *gin.Context) {
	var req SymbolSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	opts := &codesearch.SymbolSearchOptions{
		Query:      req.Query,
		SymbolType: codesearch.CodeSymbolType(req.SymbolType),
		Language:   req.Language,
		MaxResults: req.MaxResults,
	}

	result, err := h.aceService.SearchSymbols(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "符号搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: result})
}

// FindDefinitionRequest 查找定义请求
type FindDefinitionRequest struct {
	SymbolName  string `json:"symbol_name" binding:"required"`
	ContextFile string `json:"context_file,omitempty"`
}

// FindDefinition 查找符号定义
// @Summary 查找符号定义
// @Description 跳转到符号定义位置
// @Tags CodeSearch
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body FindDefinitionRequest true "查找请求"
// @Success 200 {object} common.APIResponse{data=codesearch.CodeSymbol}
// @Router /api/codesearch/definition [post]
func (h *Handler) FindDefinition(c *gin.Context) {
	var req FindDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	symbol, err := h.aceService.FindDefinition(c.Request.Context(), req.SymbolName, req.ContextFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "查找定义失败: " + err.Error()})
		return
	}

	if symbol == nil {
		c.JSON(http.StatusNotFound, common.ErrorResponse{Success: false, Message: "未找到符号定义: " + req.SymbolName})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: symbol})
}

// FindReferencesRequest 查找引用请求
type FindReferencesRequest struct {
	SymbolName string `json:"symbol_name" binding:"required"`
	MaxResults int    `json:"max_results,omitempty"`
}

// FindReferences 查找所有引用
// @Summary 查找所有引用
// @Description 查找符号的所有引用位置
// @Tags CodeSearch
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body FindReferencesRequest true "查找请求"
// @Success 200 {object} common.APIResponse{data=[]codesearch.CodeReference}
// @Router /api/codesearch/references [post]
func (h *Handler) FindReferences(c *gin.Context) {
	var req FindReferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	references, err := h.aceService.FindReferences(c.Request.Context(), req.SymbolName, maxResults)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "查找引用失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{
		Success: true,
		Data: gin.H{
			"symbol":     req.SymbolName,
			"references": references,
			"count":      len(references),
		},
	})
}

// TextSearchRequest 文本搜索请求
type TextSearchRequest struct {
	Pattern       string `json:"pattern" binding:"required"`
	FileGlob      string `json:"file_glob,omitempty"`
	IsRegex       bool   `json:"is_regex,omitempty"`
	MaxResults    int    `json:"max_results,omitempty"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
}

// TextSearch 文本搜索
// @Summary 文本搜索
// @Description 在代码库中搜索文本（支持正则表达式）
// @Tags CodeSearch
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body TextSearchRequest true "搜索请求"
// @Success 200 {object} common.APIResponse{data=[]codesearch.SearchResult}
// @Router /api/codesearch/text [post]
func (h *Handler) TextSearch(c *gin.Context) {
	var req TextSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	opts := &codesearch.TextSearchOptions{
		Pattern:       req.Pattern,
		FileGlob:      req.FileGlob,
		IsRegex:       req.IsRegex,
		MaxResults:    req.MaxResults,
		CaseSensitive: req.CaseSensitive,
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 100
	}

	results, err := h.aceService.TextSearch(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "文本搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{
		Success: true,
		Data: gin.H{
			"pattern": req.Pattern,
			"results": results,
			"count":   len(results),
		},
	})
}

// GetFileOutline 获取文件大纲
// @Summary 获取文件大纲
// @Description 获取文件的符号结构（函数、类、方法等）
// @Tags CodeSearch
// @Security BearerAuth
// @Produce json
// @Param file_path query string true "文件路径"
// @Success 200 {object} common.APIResponse{data=codesearch.FileOutline}
// @Router /api/codesearch/outline [get]
func (h *Handler) GetFileOutline(c *gin.Context) {
	filePath := c.Query("file_path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "file_path 参数必填"})
		return
	}

	outline, err := h.aceService.GetFileOutline(c.Request.Context(), filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "获取文件大纲失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: outline})
}

// SemanticSearchRequest 语义搜索请求
type SemanticSearchRequest struct {
	Query string `json:"query" binding:"required"`
	TopN  int    `json:"top_n,omitempty"`
}

// SemanticSearch 语义代码搜索
// @Summary 语义代码搜索
// @Description 基于 Embedding 的语义代码搜索
// @Tags CodeSearch
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body SemanticSearchRequest true "搜索请求"
// @Success 200 {object} common.APIResponse{data=[]codesearch.SearchResult}
// @Router /api/codesearch/semantic [post]
func (h *Handler) SemanticSearch(c *gin.Context) {
	if h.codebaseService == nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "语义搜索服务未初始化"})
		return
	}

	var req SemanticSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	topN := req.TopN
	if topN <= 0 {
		topN = 10
	}

	results, err := h.codebaseService.Search(c.Request.Context(), req.Query, topN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "语义搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, common.APIResponse{
		Success: true,
		Data: gin.H{
			"query":   req.Query,
			"results": results,
			"count":   len(results),
		},
	})
}

// BuildIndexRequest 构建索引请求
type BuildIndexRequest struct {
	ForceRefresh bool `json:"force_refresh,omitempty"`
}

// BuildIndex 构建代码索引
// @Summary 构建代码索引
// @Description 重新构建代码符号索引
// @Tags CodeSearch
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body BuildIndexRequest true "构建请求"
// @Success 200 {object} common.APIResponse
// @Router /api/codesearch/index [post]
func (h *Handler) BuildIndex(c *gin.Context) {
	var req BuildIndexRequest
	_ = c.ShouldBindJSON(&req)

	// 构建 ACE 索引
	if err := h.aceService.BuildIndex(c.Request.Context(), req.ForceRefresh); err != nil {
		c.JSON(http.StatusInternalServerError, common.ErrorResponse{Success: false, Message: "构建 ACE 索引失败: " + err.Error()})
		return
	}

	// 构建语义索引（如果可用）
	var semanticIndexed bool
	if h.codebaseService != nil {
		if err := h.codebaseService.BuildIndex(c.Request.Context()); err == nil {
			semanticIndexed = true
		}
	}

	c.JSON(http.StatusOK, common.APIResponse{
		Success: true,
		Data: gin.H{
			"message":          "索引构建完成",
			"ace_indexed":      true,
			"semantic_indexed": semanticIndexed,
		},
	})
}

// GetIndexStatus 获取索引状态
// @Summary 获取索引状态
// @Description 获取代码索引的当前状态
// @Tags CodeSearch
// @Security BearerAuth
// @Produce json
// @Success 200 {object} common.APIResponse
// @Router /api/codesearch/index/status [get]
func (h *Handler) GetIndexStatus(c *gin.Context) {
	status := gin.H{
		"ace": gin.H{
			"ready": true, // ACE 使用懒加载，始终就绪
		},
	}

	if h.codebaseService != nil {
		status["semantic"] = gin.H{
			"ready":           h.codebaseService.IsIndexReady(),
			"total_chunks":    h.codebaseService.GetTotalChunks(),
			"last_index_time": h.codebaseService.GetLastIndexTime(),
		}
	}

	c.JSON(http.StatusOK, common.APIResponse{Success: true, Data: status})
}

// SetBasePath 设置搜索基础路径
// @Summary 设置搜索基础路径
// @Description 设置代码搜索的基础目录路径
// @Tags CodeSearch
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body map[string]string true "路径配置"
// @Success 200 {object} common.APIResponse
// @Router /api/codesearch/config/base-path [put]
func (h *Handler) SetBasePath(c *gin.Context) {
	var req struct {
		BasePath string `json:"base_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.ErrorResponse{Success: false, Message: "请求参数错误: " + err.Error()})
		return
	}

	h.aceService.SetBasePath(req.BasePath)
	if h.codebaseService != nil {
		h.codebaseService.SetBasePath(req.BasePath)
	}

	c.JSON(http.StatusOK, common.APIResponse{
		Success: true,
		Data: gin.H{
			"message":   "基础路径已更新",
			"base_path": req.BasePath,
		},
	})
}
