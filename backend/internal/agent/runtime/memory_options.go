package runtime

// MemoryOptions 记忆配置选项
// 用于在 Registry 与 ContextManager/maybeSummarizeSession 之间传递统一的记忆策略
type MemoryOptions struct {
	// Mode 记忆模式: full/window/summary
	Mode string

	// HistoryLimit 历史窗口大小(条数). <=0 表示全量历史
	HistoryLimit int

	// SummaryEnabled 是否启用摘要记忆
	SummaryEnabled bool

	// SummaryTriggerMessages 触发摘要所需最小历史消息条数
	SummaryTriggerMessages int

	// SummaryMaxTokens 摘要最大 Token 数
	SummaryMaxTokens int
}
