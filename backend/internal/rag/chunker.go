package rag

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Chunker 文档分块器
type Chunker struct {
	ChunkSize    int // 分块大小(字符数)
	ChunkOverlap int // 重叠大小(字符数)
}

// NewChunker 创建新的分块器
// chunkSize: 每个分块的字符数
// chunkOverlap: 相邻分块之间的重叠字符数
func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	if chunkSize <= 0 {
		chunkSize = 500 // 默认500字符
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 10 // 重叠不超过10%
	}

	return &Chunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
	}
}

// ChunkResult 分块结果
type ChunkResult struct {
	Content     string                 // 分块内容
	ChunkIndex  int                    // 分块索引(从0开始)
	StartOffset int                    // 起始偏移量(字符)
	EndOffset   int                    // 结束偏移量(字符)
	TokenCount  int                    // Token数量(近似)
	ContentHash string                 // 内容哈希(SHA256)
	Metadata    map[string]interface{} // 元数据 (章节标题等)
}

// ChunkDocument 对文档进行分块
// content: 文档内容
// 返回: 分块结果列表
func (c *Chunker) ChunkDocument(content string) ([]*ChunkResult, error) {
	if content == "" {
		return nil, fmt.Errorf("文档内容不能为空")
	}

	// 规范化文本(去除多余空白)
	content = normalizeText(content)

	// 按句子分割
	sentences := splitIntoSentences(content)
	if len(sentences) == 0 {
		return nil, fmt.Errorf("文档没有有效句子")
	}

	chunks := make([]*ChunkResult, 0)
	currentChunk := ""
	currentStart := 0
	chunkIndex := 0

	for _, sentence := range sentences {
		// 如果当前分块加上新句子超过大小限制
		if len(currentChunk)+len(sentence) > c.ChunkSize && currentChunk != "" {
			// 保存当前分块
			chunk := c.createChunk(currentChunk, chunkIndex, currentStart, currentStart+len(currentChunk))
			chunks = append(chunks, chunk)

			// 开始新分块,保留重叠部分
			overlap := c.getOverlapText(currentChunk)
			currentChunk = overlap + sentence
			currentStart = currentStart + len(currentChunk) - len(overlap) - len(sentence)
			chunkIndex++
		} else {
			// 添加到当前分块
			if currentChunk != "" {
				currentChunk += " "
			}
			currentChunk += sentence
		}
	}

	// 保存最后一个分块
	if currentChunk != "" {
		chunk := c.createChunk(currentChunk, chunkIndex, currentStart, currentStart+len(currentChunk))
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// createChunk 创建分块结果
func (c *Chunker) createChunk(content string, index, start, end int) *ChunkResult {
	return &ChunkResult{
		Content:     strings.TrimSpace(content),
		ChunkIndex:  index,
		StartOffset: start,
		EndOffset:   end,
		TokenCount:  estimateTokenCount(content),
		ContentHash: hashContent(content),
	}
}

// getOverlapText 获取重叠文本
// 从文本末尾获取指定长度的重叠部分
func (c *Chunker) getOverlapText(text string) string {
	if c.ChunkOverlap == 0 || len(text) <= c.ChunkOverlap {
		return ""
	}

	// 从末尾向前取重叠长度的文本
	overlap := text[len(text)-c.ChunkOverlap:]

	// 尝试从完整单词开始
	if idx := strings.Index(overlap, " "); idx > 0 {
		overlap = overlap[idx+1:]
	}

	return overlap
}

// normalizeText 规范化文本
// 去除多余空白、换行符等
func normalizeText(text string) string {
	// 替换多个空白为单个空格
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

// splitIntoSentences 将文本分割成句子
// 使用简单的规则: 以句号、问号、感叹号结尾
func splitIntoSentences(text string) []string {
	sentences := make([]string, 0)
	current := ""

	runes := []rune(text)
	for i, r := range runes {
		current += string(r)

		// 检查是否是句子结束标记
		if r == '。' || r == '!' || r == '?' || r == '.' {
			// 确保不是数字中的小数点
			if r == '.' && i+1 < len(runes) {
				next := runes[i+1]
				if next >= '0' && next <= '9' {
					continue
				}
			}

			// 添加句子
			sentence := strings.TrimSpace(current)
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			current = ""
		}
	}

	// 添加最后剩余的内容
	if current != "" {
		sentence := strings.TrimSpace(current)
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// estimateTokenCount 估算Token数量
// 简单规则: 英文按单词数, 中文按字符数/1.5
func estimateTokenCount(text string) int {
	// 统计英文单词数
	words := strings.Fields(text)
	wordCount := len(words)

	// 统计中文字符数
	chineseCount := 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FA5 { // 基本汉字Unicode范围
			chineseCount++
		}
	}

	// 中文字符约1.5个字符=1个token
	// 英文单词约1.3个字符=1个token
	tokens := wordCount + int(float64(chineseCount)/1.5)

	return tokens
}

// hashContent 计算内容哈希
func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// ChunkByFixedSize 按固定大小分块(简单方法)
// 不考虑句子边界,直接按字符数切分
func ChunkByFixedSize(content string, size, overlap int) []*ChunkResult {
	if content == "" || size <= 0 {
		return nil
	}

	chunks := make([]*ChunkResult, 0)
	runes := []rune(content)
	totalLen := len(runes)
	index := 0

	for start := 0; start < totalLen; start += size - overlap {
		end := start + size
		if end > totalLen {
			end = totalLen
		}

		chunkContent := string(runes[start:end])
		chunk := &ChunkResult{
			Content:     strings.TrimSpace(chunkContent),
			ChunkIndex:  index,
			StartOffset: start,
			EndOffset:   end,
			TokenCount:  estimateTokenCount(chunkContent),
			ContentHash: hashContent(chunkContent),
		}

		chunks = append(chunks, chunk)
		index++

		// 如果已经到达末尾,退出
		if end >= totalLen {
			break
		}
	}

	return chunks
}

// GetChunkSummary 获取分块摘要信息
func GetChunkSummary(chunks []*ChunkResult) string {
	if len(chunks) == 0 {
		return "无分块"
	}

	totalChars := 0
	totalTokens := 0
	for _, chunk := range chunks {
		totalChars += utf8.RuneCountInString(chunk.Content)
		totalTokens += chunk.TokenCount
	}

	avgChars := totalChars / len(chunks)
	avgTokens := totalTokens / len(chunks)

	return fmt.Sprintf("分块数: %d, 总字符数: %d, 总Token数: %d, 平均字符数: %d, 平均Token数: %d",
		len(chunks), totalChars, totalTokens, avgChars, avgTokens)
}
