// Package cache 提供缓存相关功能
package cache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// LFUNode 表示 LFU 缓存中的一个节点
type LFUNode struct {
	key       string
	frequency int       // 访问频率
	timestamp time.Time // 最后访问时间，用于频率相同时的 FIFO 淘汰
}

// LFUCache LFU 缓存淘汰策略实现
type LFUCache struct {
	diskCache   *DiskCache               // 底层磁盘缓存
	capacity    int                      // 最大容量
	minFreq     int                      // 当前最小频率
	keyToNode   map[string]*LFUNode      // key -> node 映射
	freqToList  map[int]*list.List       // 频率 -> 双向链表映射
	nodeToElem  map[*LFUNode]*list.Element // node -> list.Element 映射
	mu          sync.RWMutex             // 读写锁
}

// NewLFUCache 创建 LFU 缓存实例
// capacity: 内存中维护的最大条目数（用于频率统计）
// diskCache: 底层磁盘缓存
func NewLFUCache(capacity int, diskCache *DiskCache) *LFUCache {
	return &LFUCache{
		diskCache:  diskCache,
		capacity:   capacity,
		minFreq:    0,
		keyToNode:  make(map[string]*LFUNode),
		freqToList: make(map[int]*list.List),
		nodeToElem: make(map[*LFUNode]*list.Element),
	}
}

// Get 获取缓存条目
func (lfu *LFUCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	// 先从底层磁盘缓存获取
	entry, err := lfu.diskCache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	if entry != nil {
		// 更新 LFU 频率统计
		lfu.updateFrequency(key)
	}
	
	return entry, nil
}

// Set 设置缓存条目
func (lfu *LFUCache) Set(ctx context.Context, entry *CacheEntry) error {
	// 写入底层磁盘缓存
	if err := lfu.diskCache.Set(ctx, entry); err != nil {
		return err
	}
	
	lfu.mu.Lock()
	defer lfu.mu.Unlock()
	
	// 如果已存在，更新频率
	if node, exists := lfu.keyToNode[entry.CacheKey]; exists {
		lfu.increaseFrequency(node)
		return nil
	}
	
	// 如果达到容量上限，淘汰最不常用的条目
	if len(lfu.keyToNode) >= lfu.capacity {
		lfu.evict(ctx)
	}
	
	// 添加新节点
	node := &LFUNode{
		key:       entry.CacheKey,
		frequency: 1,
		timestamp: time.Now(),
	}
	
	lfu.keyToNode[entry.CacheKey] = node
	lfu.addToFreqList(node)
	lfu.minFreq = 1
	
	return nil
}

// Delete 删除缓存条目
func (lfu *LFUCache) Delete(ctx context.Context, key string) error {
	// 从底层磁盘缓存删除
	if err := lfu.diskCache.Delete(ctx, key); err != nil {
		return err
	}
	
	lfu.mu.Lock()
	defer lfu.mu.Unlock()
	
	// 从 LFU 结构中删除
	if node, exists := lfu.keyToNode[key]; exists {
		lfu.removeNode(node)
		delete(lfu.keyToNode, key)
	}
	
	return nil
}

// Clear 清空所有缓存
func (lfu *LFUCache) Clear(ctx context.Context) error {
	// 清空底层磁盘缓存
	if err := lfu.diskCache.Clear(ctx); err != nil {
		return err
	}
	
	lfu.mu.Lock()
	defer lfu.mu.Unlock()
	
	// 清空 LFU 结构
	lfu.keyToNode = make(map[string]*LFUNode)
	lfu.freqToList = make(map[int]*list.List)
	lfu.nodeToElem = make(map[*LFUNode]*list.Element)
	lfu.minFreq = 0
	
	return nil
}

// GetStats 获取缓存统计（委托给底层 DiskCache）
func (lfu *LFUCache) GetStats(ctx context.Context) (map[string]any, error) {
	stats, err := lfu.diskCache.GetStats(ctx)
	if err != nil {
		return nil, err
	}
	
	lfu.mu.RLock()
	defer lfu.mu.RUnlock()
	
	// 添加 LFU 特有的统计信息
	stats["lfu_tracked_entries"] = len(lfu.keyToNode)
	stats["lfu_min_frequency"] = lfu.minFreq
	stats["lfu_capacity"] = lfu.capacity
	
	// 统计频率分布
	freqDistribution := make(map[int]int)
	for _, node := range lfu.keyToNode {
		freqDistribution[node.frequency]++
	}
	stats["frequency_distribution"] = freqDistribution
	
	return stats, nil
}

// Close 关闭缓存
func (lfu *LFUCache) Close() error {
	return lfu.diskCache.Close()
}

// updateFrequency 更新条目的访问频率（读取时调用）
func (lfu *LFUCache) updateFrequency(key string) {
	lfu.mu.Lock()
	defer lfu.mu.Unlock()
	
	node, exists := lfu.keyToNode[key]
	if !exists {
		// 如果不存在，添加新节点
		if len(lfu.keyToNode) >= lfu.capacity {
			lfu.evict(context.Background())
		}
		
		node = &LFUNode{
			key:       key,
			frequency: 1,
			timestamp: time.Now(),
		}
		lfu.keyToNode[key] = node
		lfu.addToFreqList(node)
		lfu.minFreq = 1
		return
	}
	
	// 增加频率
	lfu.increaseFrequency(node)
}

// increaseFrequency 增加节点频率
func (lfu *LFUCache) increaseFrequency(node *LFUNode) {
	// 从当前频率列表中移除
	lfu.removeNode(node)
	
	// 更新频率和时间戳
	node.frequency++
	node.timestamp = time.Now()
	
	// 添加到新频率列表
	lfu.addToFreqList(node)
	
	// 更新最小频率
	if lfu.freqToList[lfu.minFreq] == nil || lfu.freqToList[lfu.minFreq].Len() == 0 {
		lfu.minFreq++
	}
}

// addToFreqList 将节点添加到频率列表
func (lfu *LFUCache) addToFreqList(node *LFUNode) {
	freq := node.frequency
	
	if lfu.freqToList[freq] == nil {
		lfu.freqToList[freq] = list.New()
	}
	
	elem := lfu.freqToList[freq].PushBack(node)
	lfu.nodeToElem[node] = elem
}

// removeNode 从频率列表中移除节点
func (lfu *LFUCache) removeNode(node *LFUNode) {
	freq := node.frequency
	elem := lfu.nodeToElem[node]
	
	if elem != nil && lfu.freqToList[freq] != nil {
		lfu.freqToList[freq].Remove(elem)
		delete(lfu.nodeToElem, node)
		
		// 如果该频率的列表为空，删除该列表
		if lfu.freqToList[freq].Len() == 0 {
			delete(lfu.freqToList, freq)
		}
	}
}

// evict 淘汰最不常用的条目
func (lfu *LFUCache) evict(ctx context.Context) {
	if lfu.minFreq == 0 || lfu.freqToList[lfu.minFreq] == nil {
		return
	}
	
	// 获取最小频率列表的第一个元素（最早添加）
	minFreqList := lfu.freqToList[lfu.minFreq]
	if minFreqList.Len() == 0 {
		return
	}
	
	elem := minFreqList.Front()
	if elem == nil {
		return
	}
	
	node := elem.Value.(*LFUNode)
	
	// 从底层缓存删除
	_ = lfu.diskCache.Delete(ctx, node.key)
	
	// 从 LFU 结构中删除
	lfu.removeNode(node)
	delete(lfu.keyToNode, node.key)
}

// RecordCacheHit 记录缓存命中（委托给底层 DiskCache）
func (lfu *LFUCache) RecordCacheHit() {
	lfu.diskCache.RecordCacheHit()
}

// RecordCacheMiss 记录缓存未命中（委托给底层 DiskCache）
func (lfu *LFUCache) RecordCacheMiss() {
	lfu.diskCache.RecordCacheMiss()
}

// GetMetrics 获取缓存指标（委托给底层 DiskCache）
func (lfu *LFUCache) GetMetrics() *DiskCacheMetrics {
	return lfu.diskCache.GetMetrics()
}
