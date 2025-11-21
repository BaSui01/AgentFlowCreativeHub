package workflows

import (
	"fmt"
	"net/http"

	"backend/internal/workflow"
	"backend/internal/workflow/template"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TemplateHandler 工作流模板处理器
type TemplateHandler struct {
	db               *gorm.DB
	templateLoader   *template.TemplateLoader
	capabilityLoader *template.AgentCapabilityLoader
	validator        *workflow.Validator
}

// NewTemplateHandler 创建模板处理器
func NewTemplateHandler(
	db *gorm.DB,
	templateLoader *template.TemplateLoader,
	capabilityLoader *template.AgentCapabilityLoader,
) *TemplateHandler {
	return &TemplateHandler{
		db:               db,
		templateLoader:   templateLoader,
		capabilityLoader: capabilityLoader,
		validator:        workflow.NewValidator(capabilityLoader),
	}
}

// ListTemplates 获取模板列表
// GET /api/v1/workflows/templates
func (h *TemplateHandler) ListTemplates(c *gin.Context) {
	category := c.Query("category")

	var templates map[string]*template.Template
	if category != "" {
		templateList := h.templateLoader.ListByCategory(category)
		templates = make(map[string]*template.Template)
		for _, t := range templateList {
			templates[t.Category] = t
		}
	} else {
		templates = h.templateLoader.ListTemplates()
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}

// GetTemplate 获取模板详情
// GET /api/v1/workflows/templates/:key
func (h *TemplateHandler) GetTemplate(c *gin.Context) {
	key := c.Param("key")

	tmpl, err := h.templateLoader.GetTemplate(key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, tmpl)
}

// QuickCreate 快速创建工作流（基于模板）
// POST /api/v1/workflows/quick-create
func (h *TemplateHandler) QuickCreate(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var req struct {
		Template string         `json:"template" binding:"required"`
		Name     string         `json:"name" binding:"required"`
		Config   map[string]any `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 实例化模板
	definition, err := h.templateLoader.InstantiateTemplate(req.Template, req.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	workflowDef, err := mapToWorkflowDefinition(definition)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 创建工作流
	workflowID := uuid.New().String()
	wf := &workflow.Workflow{
		ID:          workflowID,
		TenantID:    tenantID,
		OwnerUserID: userID,
		Name:        req.Name,
		Description: "基于模板 " + req.Template + " 创建",
		Definition:  workflowDef,
		Version:     "1.0",
		Visibility:  "personal",
		CreatedBy:   userID,
	}

	if err := h.db.WithContext(c.Request.Context()).Create(wf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建工作流失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow_id": workflowID,
		"name":        req.Name,
		"template":    req.Template,
	})
}

// CustomCreate 自定义创建工作流（Agent 链）
// POST /api/v1/workflows/custom-create
func (h *TemplateHandler) CustomCreate(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var req struct {
		Name           string           `json:"name" binding:"required"`
		Description    string           `json:"description"`
		AgentChain     []map[string]any `json:"agent_chain" binding:"required"`
		AutomationMode string           `json:"automation_mode"` // full_auto, semi_auto, manual
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 默认自动化模式
	if req.AutomationMode == "" {
		req.AutomationMode = "semi_auto"
	}

	// 构建工作流定义
	steps := make([]map[string]any, 0, len(req.AgentChain))
	for i, agentConfig := range req.AgentChain {
		stepID := fmt.Sprintf("step_%d", i+1)

		step := map[string]any{
			"id":           stepID,
			"type":         "agent",
			"agent_type":   agentConfig["agent_type"],
			"auto_execute": true,
		}

		// 可选字段
		if role, ok := agentConfig["role"].(string); ok && role != "" {
			step["role"] = role
		}
		if systemPrompt, ok := agentConfig["system_prompt_override"].(string); ok && systemPrompt != "" {
			step["system_prompt_override"] = systemPrompt
		}
		if approvalRequired, ok := agentConfig["approval_required"].(bool); ok {
			step["approval_required"] = approvalRequired
		}
		if input, ok := agentConfig["input"].(map[string]any); ok {
			step["input"] = input
		}

		// 设置依赖（除了第一个步骤）
		if i > 0 {
			step["depends_on"] = []string{fmt.Sprintf("step_%d", i)}
		}

		steps = append(steps, step)
	}

	// 构建完整定义
	definition := map[string]any{
		"name":    req.Name,
		"version": "1.0",
		"automation_config": map[string]any{
			"mode": req.AutomationMode,
		},
		"steps": steps,
	}

	// 创建工作流
	workflowDef, err := mapToWorkflowDefinition(definition)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workflowID := uuid.New().String()
	wf := &workflow.Workflow{
		ID:          workflowID,
		TenantID:    tenantID,
		OwnerUserID: userID,
		Name:        req.Name,
		Description: req.Description,
		Definition:  workflowDef,
		Version:     "1.0",
		Visibility:  "personal",
		CreatedBy:   userID,
	}

	if err := h.db.WithContext(c.Request.Context()).Create(wf).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建工作流失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow_id": workflowID,
		"name":        req.Name,
		"steps":       len(steps),
	})
}

// GetAgentCapabilities 获取 Agent 能力目录
// @Summary 获取 Agent 能力目录
// @Tags Agents
// @Security BearerAuth
// @Produce json
// @Param agent_type query string false "Agent 类型"
// @Success 200 {object} map[string]any
// @Router /api/agents/capabilities [get]
// @Router /api/v1/agents/capabilities [get]
func (h *TemplateHandler) GetAgentCapabilities(c *gin.Context) {
	agentType := c.Query("agent_type")

	if agentType != "" {
		// 获取指定 Agent 的能力
		capabilities, err := h.capabilityLoader.GetCapabilities(agentType)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, capabilities)
	} else {
		// 获取所有 Agent 的能力
		capabilities := h.capabilityLoader.ListAllCapabilities()

		c.JSON(http.StatusOK, gin.H{
			"capabilities": capabilities,
			"total":        len(capabilities),
		})
	}
}

// GetRoleCapability 获取指定角色的能力详情
// @Summary 获取 Agent 角色能力详情
// @Tags Agents
// @Security BearerAuth
// @Produce json
// @Param agent_type path string true "Agent 类型"
// @Param role path string true "角色标识"
// @Success 200 {object} template.AgentRoleCapability
// @Failure 404 {object} map[string]string
// @Router /api/agents/capabilities/{agent_type}/{role} [get]
// @Router /api/v1/agents/capabilities/{agent_type}/{role} [get]
func (h *TemplateHandler) GetRoleCapability(c *gin.Context) {
	agentType := c.Param("agent_type")
	role := c.Param("role")

	capability, err := h.capabilityLoader.GetRoleCapability(agentType, role)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, capability)
}

// ValidateWorkflowDefinition 验证工作流定义
// POST /api/v1/workflows/validate
func (h *TemplateHandler) ValidateWorkflowDefinition(c *gin.Context) {
	var req struct {
		Definition map[string]any `json:"definition" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 验证定义
	errors := h.validator.Validate(req.Definition)

	if len(errors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid":  false,
			"errors": errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "工作流定义有效",
	})
}
