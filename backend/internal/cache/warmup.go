package cache

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WarmupConfig 缓存预热配置
type WarmupConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Concurrency     int           `yaml:"concurrency"`       // 并发预热数
	Timeout         time.Duration `yaml:"timeout"`           // 单项预热超时
	RetryAttempts   int           `yaml:"retry_attempts"`    // 重试次数
	RetryDelay      time.Duration `yaml:"retry_delay"`       // 重试间隔
}

// DefaultWarmupConfig 默认预热配置
func DefaultWarmupConfig() *WarmupConfig {
	return &WarmupConfig{
		Enabled:       true,
		Concurrency:   5,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
	}
}

// WarmupItem 预热项
type WarmupItem struct {
	Key      string                                  // 缓存键
	Loader   func(ctx context.Context) (any, error)  // 数据加载函数
	TTL      time.Duration                           // 缓存过期时间
	Priority int                                     // 优先级（数字越大优先级越高）
}

// WarmupResult 预热结果
type WarmupResult struct {
	Key       string
	Success   bool
	Error     error
	Duration  time.Duration
	Retries   int
}

// CacheWarmer 缓存预热器
type CacheWarmer struct {
	config   *WarmupConfig
	logger   *zap.Logger
	items    []WarmupItem
	mu       sync.RWMutex
	storage  WarmupStorage
}

// WarmupStorage 预热存储接口
type WarmupStorage interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (any, bool, error)
}

// NewCacheWarmer 创建缓存预热器
func NewCacheWarmer(config *WarmupConfig, storage WarmupStorage, logger *zap.Logger) *CacheWarmer {
	if config == nil {
		config = DefaultWarmupConfig()
	}
	return &CacheWarmer{
		config:  config,
		logger:  logger,
		items:   make([]WarmupItem, 0),
		storage: storage,
	}
}

// Register 注册预热项
func (w *CacheWarmer) Register(item WarmupItem) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = append(w.items, item)
}

// RegisterBatch 批量注册预热项
func (w *CacheWarmer) RegisterBatch(items []WarmupItem) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = append(w.items, items...)
}

// Warmup 执行预热
func (w *CacheWarmer) Warmup(ctx context.Context) []WarmupResult {
	if !w.config.Enabled {
		w.logger.Info("缓存预热已禁用")
		return nil
	}

	w.mu.RLock()
	items := make([]WarmupItem, len(w.items))
	copy(items, w.items)
	w.mu.RUnlock()

	if len(items) == 0 {
		w.logger.Info("没有需要预热的缓存项")
		return nil
	}

	// 按优先级排序
	sortByPriority(items)

	w.logger.Info("开始缓存预热",
		zap.Int("total_items", len(items)),
		zap.Int("concurrency", w.config.Concurrency),
	)

	startTime := time.Now()
	results := w.executeWarmup(ctx, items)
	duration := time.Since(startTime)

	// 统计结果
	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	w.logger.Info("缓存预热完成",
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
		zap.Duration("duration", duration),
	)

	return results
}

// executeWarmup 并发执行预热
func (w *CacheWarmer) executeWarmup(ctx context.Context, items []WarmupItem) []WarmupResult {
	results := make([]WarmupResult, len(items))
	
	// 使用信号量控制并发
	sem := make(chan struct{}, w.config.Concurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, it WarmupItem) {
			defer wg.Done()
			
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = w.warmupItem(ctx, it)
		}(i, item)
	}

	wg.Wait()
	return results
}

// warmupItem 预热单个项
func (w *CacheWarmer) warmupItem(ctx context.Context, item WarmupItem) WarmupResult {
	result := WarmupResult{Key: item.Key}
	startTime := time.Now()

	// 重试逻辑
	var lastErr error
	for attempt := 0; attempt <= w.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			result.Retries = attempt
			time.Sleep(w.config.RetryDelay)
		}

		// 创建带超时的上下文
		itemCtx, cancel := context.WithTimeout(ctx, w.config.Timeout)
		
		// 加载数据
		value, err := item.Loader(itemCtx)
		cancel()

		if err != nil {
			lastErr = err
			w.logger.Warn("缓存预热加载失败",
				zap.String("key", item.Key),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			continue
		}

		// 写入缓存
		if err := w.storage.Set(ctx, item.Key, value, item.TTL); err != nil {
			lastErr = err
			w.logger.Warn("缓存预热写入失败",
				zap.String("key", item.Key),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			continue
		}

		// 成功
		result.Success = true
		result.Duration = time.Since(startTime)
		w.logger.Debug("缓存预热成功",
			zap.String("key", item.Key),
			zap.Duration("duration", result.Duration),
		)
		return result
	}

	// 所有重试都失败
	result.Error = lastErr
	result.Duration = time.Since(startTime)
	return result
}

// Clear 清除所有注册的预热项
func (w *CacheWarmer) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = make([]WarmupItem, 0)
}

// GetItems 获取所有预热项（用于调试）
func (w *CacheWarmer) GetItems() []WarmupItem {
	w.mu.RLock()
	defer w.mu.RUnlock()
	items := make([]WarmupItem, len(w.items))
	copy(items, w.items)
	return items
}

// sortByPriority 按优先级排序（高优先级在前）
func sortByPriority(items []WarmupItem) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Priority > items[i].Priority {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// MemoryWarmupStorage 内存预热存储（用于测试）
type MemoryWarmupStorage struct {
	data map[string]any
	mu   sync.RWMutex
}

// NewMemoryWarmupStorage 创建内存预热存储
func NewMemoryWarmupStorage() *MemoryWarmupStorage {
	return &MemoryWarmupStorage{
		data: make(map[string]any),
	}
}

func (s *MemoryWarmupStorage) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *MemoryWarmupStorage) Get(ctx context.Context, key string) (any, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	return value, ok, nil
}
