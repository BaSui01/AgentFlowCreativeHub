package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ABTestService Agent A/B 测试服务
type ABTestService struct {
	experiments map[string]*Experiment
	results     map[string]*ExperimentResults
	mu          sync.RWMutex
}

// Experiment A/B 测试实验
type Experiment struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Status      ExperimentStatus    `json:"status"`
	Variants    []Variant           `json:"variants"`
	TrafficSplit map[string]float64 `json:"traffic_split"` // variant_id -> 流量百分比
	Metrics     []string            `json:"metrics"`       // 需要跟踪的指标
	StartTime   *time.Time          `json:"start_time,omitempty"`
	EndTime     *time.Time          `json:"end_time,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	CreatedBy   string              `json:"created_by"`
}

// ExperimentStatus 实验状态
type ExperimentStatus string

const (
	StatusDraft     ExperimentStatus = "draft"
	StatusRunning   ExperimentStatus = "running"
	StatusPaused    ExperimentStatus = "paused"
	StatusCompleted ExperimentStatus = "completed"
)

// Variant 实验变体
type Variant struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	AgentConfig map[string]any `json:"agent_config"` // Agent 配置覆盖
	IsControl   bool           `json:"is_control"`   // 是否为对照组
}

// ExperimentResults 实验结果
type ExperimentResults struct {
	ExperimentID string                    `json:"experiment_id"`
	VariantStats map[string]*VariantStats  `json:"variant_stats"`
	StartTime    time.Time                 `json:"start_time"`
	LastUpdated  time.Time                 `json:"last_updated"`
}

// VariantStats 变体统计
type VariantStats struct {
	VariantID    string             `json:"variant_id"`
	SampleSize   int64              `json:"sample_size"`
	Conversions  int64              `json:"conversions"`
	ConvRate     float64            `json:"conversion_rate"`
	Metrics      map[string]float64 `json:"metrics"`
	AvgLatencyMs float64            `json:"avg_latency_ms"`
	SuccessRate  float64            `json:"success_rate"`
	AvgTokens    float64            `json:"avg_tokens"`
}

// NewABTestService 创建 A/B 测试服务
func NewABTestService() *ABTestService {
	return &ABTestService{
		experiments: make(map[string]*Experiment),
		results:     make(map[string]*ExperimentResults),
	}
}

// CreateExperiment 创建实验
func (s *ABTestService) CreateExperiment(exp *Experiment) error {
	if exp.ID == "" {
		exp.ID = fmt.Sprintf("exp_%d", time.Now().UnixNano())
	}

	// 验证流量分配
	var totalTraffic float64
	for _, pct := range exp.TrafficSplit {
		totalTraffic += pct
	}
	if totalTraffic > 1.0 {
		return fmt.Errorf("total traffic split exceeds 100%%")
	}

	exp.Status = StatusDraft
	exp.CreatedAt = time.Now()

	s.mu.Lock()
	s.experiments[exp.ID] = exp
	s.results[exp.ID] = &ExperimentResults{
		ExperimentID: exp.ID,
		VariantStats: make(map[string]*VariantStats),
		StartTime:    time.Now(),
		LastUpdated:  time.Now(),
	}
	for _, v := range exp.Variants {
		s.results[exp.ID].VariantStats[v.ID] = &VariantStats{
			VariantID: v.ID,
			Metrics:   make(map[string]float64),
		}
	}
	s.mu.Unlock()

	return nil
}

// StartExperiment 启动实验
func (s *ABTestService) StartExperiment(expID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment not found: %s", expID)
	}

	if exp.Status == StatusRunning {
		return fmt.Errorf("experiment already running")
	}

	now := time.Now()
	exp.Status = StatusRunning
	exp.StartTime = &now
	s.results[expID].StartTime = now

	return nil
}

// PauseExperiment 暂停实验
func (s *ABTestService) PauseExperiment(expID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment not found: %s", expID)
	}

	exp.Status = StatusPaused
	return nil
}

// CompleteExperiment 结束实验
func (s *ABTestService) CompleteExperiment(expID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, ok := s.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment not found: %s", expID)
	}

	now := time.Now()
	exp.Status = StatusCompleted
	exp.EndTime = &now

	return nil
}

// GetVariant 根据用户 ID 获取分配的变体
func (s *ABTestService) GetVariant(expID, userID string) (*Variant, error) {
	s.mu.RLock()
	exp, ok := s.experiments[expID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("experiment not found: %s", expID)
	}

	if exp.Status != StatusRunning {
		return nil, fmt.Errorf("experiment not running")
	}

	// 使用一致性哈希分配变体
	bucket := s.hashToBucket(expID, userID)
	
	var cumulative float64
	for _, variant := range exp.Variants {
		cumulative += exp.TrafficSplit[variant.ID]
		if bucket < cumulative {
			return &variant, nil
		}
	}

	// 返回对照组
	for _, variant := range exp.Variants {
		if variant.IsControl {
			return &variant, nil
		}
	}

	return &exp.Variants[0], nil
}

// hashToBucket 一致性哈希到 [0, 1) 区间
func (s *ABTestService) hashToBucket(expID, userID string) float64 {
	h := sha256.New()
	h.Write([]byte(expID + ":" + userID))
	hash := hex.EncodeToString(h.Sum(nil))
	
	// 取前 8 字节转换为数值
	var val uint64
	for i := 0; i < 8 && i < len(hash); i++ {
		val = val*16 + uint64(hash[i]%16)
	}
	
	return float64(val) / float64(1<<32)
}

// RecordResult 记录实验结果
func (s *ABTestService) RecordResult(expID, variantID string, result *TrialResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	results, ok := s.results[expID]
	if !ok {
		return fmt.Errorf("experiment not found: %s", expID)
	}

	stats, ok := results.VariantStats[variantID]
	if !ok {
		return fmt.Errorf("variant not found: %s", variantID)
	}

	// 更新统计
	stats.SampleSize++
	if result.Success {
		stats.Conversions++
	}
	stats.ConvRate = float64(stats.Conversions) / float64(stats.SampleSize)

	// 更新延迟（增量平均）
	n := float64(stats.SampleSize)
	stats.AvgLatencyMs = stats.AvgLatencyMs*(n-1)/n + float64(result.LatencyMs)/n
	stats.AvgTokens = stats.AvgTokens*(n-1)/n + float64(result.Tokens)/n

	successCount := stats.ConvRate * float64(stats.SampleSize)
	stats.SuccessRate = successCount / float64(stats.SampleSize)

	// 更新自定义指标
	for k, v := range result.Metrics {
		old := stats.Metrics[k]
		stats.Metrics[k] = old*(n-1)/n + v/n
	}

	results.LastUpdated = time.Now()

	return nil
}

// TrialResult 单次试验结果
type TrialResult struct {
	Success   bool               `json:"success"`
	LatencyMs int64              `json:"latency_ms"`
	Tokens    int64              `json:"tokens"`
	Metrics   map[string]float64 `json:"metrics"`
}

// GetResults 获取实验结果
func (s *ABTestService) GetResults(expID string) (*ExperimentResults, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results, ok := s.results[expID]
	if !ok {
		return nil, fmt.Errorf("experiment not found: %s", expID)
	}

	return results, nil
}

// GetExperiment 获取实验详情
func (s *ABTestService) GetExperiment(expID string) (*Experiment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exp, ok := s.experiments[expID]
	if !ok {
		return nil, fmt.Errorf("experiment not found: %s", expID)
	}

	return exp, nil
}

// ListExperiments 列出所有实验
func (s *ABTestService) ListExperiments(status ExperimentStatus) []*Experiment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Experiment, 0)
	for _, exp := range s.experiments {
		if status == "" || exp.Status == status {
			result = append(result, exp)
		}
	}
	return result
}

// DeleteExperiment 删除实验
func (s *ABTestService) DeleteExperiment(expID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.experiments[expID]; !ok {
		return fmt.Errorf("experiment not found: %s", expID)
	}

	delete(s.experiments, expID)
	delete(s.results, expID)
	return nil
}

// CalculateSignificance 计算统计显著性（简化版 Z-test）
func (s *ABTestService) CalculateSignificance(expID string) (*SignificanceResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exp, ok := s.experiments[expID]
	if !ok {
		return nil, fmt.Errorf("experiment not found")
	}

	results, ok := s.results[expID]
	if !ok {
		return nil, fmt.Errorf("results not found")
	}

	// 找到对照组
	var controlID string
	for _, v := range exp.Variants {
		if v.IsControl {
			controlID = v.ID
			break
		}
	}
	if controlID == "" {
		controlID = exp.Variants[0].ID
	}

	controlStats := results.VariantStats[controlID]
	if controlStats == nil || controlStats.SampleSize < 30 {
		return nil, fmt.Errorf("insufficient control sample size")
	}

	sig := &SignificanceResult{
		ControlID:     controlID,
		Comparisons:   make([]VariantComparison, 0),
	}

	for variantID, stats := range results.VariantStats {
		if variantID == controlID || stats.SampleSize < 30 {
			continue
		}

		// 计算 Z-score
		p1 := controlStats.ConvRate
		p2 := stats.ConvRate
		n1 := float64(controlStats.SampleSize)
		n2 := float64(stats.SampleSize)

		pooledP := (p1*n1 + p2*n2) / (n1 + n2)
		se := pooledSE(pooledP, n1, n2)
		
		var zScore float64
		if se > 0 {
			zScore = (p2 - p1) / se
		}

		// 简化的显著性判断
		significant := abs(zScore) > 1.96 // 95% 置信度

		sig.Comparisons = append(sig.Comparisons, VariantComparison{
			VariantID:   variantID,
			Improvement: (p2 - p1) / p1 * 100,
			ZScore:      zScore,
			Significant: significant,
		})
	}

	return sig, nil
}

func pooledSE(p, n1, n2 float64) float64 {
	if n1 <= 0 || n2 <= 0 {
		return 0
	}
	return sqrt(p * (1 - p) * (1/n1 + 1/n2))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// SignificanceResult 显著性结果
type SignificanceResult struct {
	ControlID   string              `json:"control_id"`
	Comparisons []VariantComparison `json:"comparisons"`
}

// VariantComparison 变体对比
type VariantComparison struct {
	VariantID   string  `json:"variant_id"`
	Improvement float64 `json:"improvement_pct"`
	ZScore      float64 `json:"z_score"`
	Significant bool    `json:"significant"`
}

// ABTestMiddleware A/B 测试中间件（用于 Agent 执行）
type ABTestMiddleware struct {
	service *ABTestService
}

func NewABTestMiddleware(service *ABTestService) *ABTestMiddleware {
	return &ABTestMiddleware{service: service}
}

// WrapExecution 包装 Agent 执行，自动分配变体并记录结果
func (m *ABTestMiddleware) WrapExecution(
	ctx context.Context,
	expID, userID string,
	execute func(config map[string]any) (*TrialResult, error),
) (*TrialResult, error) {
	variant, err := m.service.GetVariant(expID, userID)
	if err != nil {
		// 实验不存在或未运行，使用默认配置
		return execute(nil)
	}

	result, err := execute(variant.AgentConfig)
	if err != nil {
		// 记录失败
		m.service.RecordResult(expID, variant.ID, &TrialResult{
			Success: false,
		})
		return nil, err
	}

	// 记录成功
	m.service.RecordResult(expID, variant.ID, result)

	return result, nil
}

// RandomVariant 随机分配变体（用于测试）
func RandomVariant(variants []Variant, weights []float64) *Variant {
	if len(variants) == 0 {
		return nil
	}

	r := rand.Float64()
	var cumulative float64
	for i, w := range weights {
		cumulative += w
		if r < cumulative && i < len(variants) {
			return &variants[i]
		}
	}
	return &variants[0]
}
