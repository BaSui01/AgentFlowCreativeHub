package rag

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// SemanticChunker 语义分块器 (基于段落/章节)
type SemanticChunker struct {
	MaxChunkSize    int     // 最大分块大小
	MinChunkSize    int     // 最小分块大小
	OverlapSize     int     // 重叠大小
	SimilarityThreshold float64 // 相似度阈值 (用于合并相似段落)
}

// NewSemanticChunker 创建语义分块器
func NewSemanticChunker(maxSize, minSize, overlap int) *SemanticChunker {
	if maxSize <= 0 {
		maxSize = 1000
	}
	if minSize <= 0 {
		minSize = 100
	}
	if overlap < 0 {
		overlap = 50
	}
	return &SemanticChunker{
		MaxChunkSize:    maxSize,
		MinChunkSize:    minSize,
		OverlapSize:     overlap,
		SimilarityThreshold: 0.5,
	}
}

// ChunkByParagraph 按段落分块
func (c *SemanticChunker) ChunkByParagraph(content string) ([]*ChunkResult, error) {
	paragraphs := c.splitIntoParagraphs(content)
	if len(paragraphs) == 0 {
		// 降级到固定大小分块
		return ChunkByFixedSize(content, c.MaxChunkSize, c.OverlapSize), nil
	}

	var chunks []*ChunkResult
	var currentChunk strings.Builder
	var currentStart int
	chunkIndex := 0

	for _, para := range paragraphs {
		paraLen := utf8.RuneCountInString(para)

		// 如果单个段落超过最大大小，需要进一步分割
		if paraLen > c.MaxChunkSize {
			// 先保存当前累积的内容
			if currentChunk.Len() > 0 {
				chunk := c.createChunk(currentChunk.String(), chunkIndex, currentStart)
				chunks = append(chunks, chunk)
				currentChunk.Reset()
				chunkIndex++
			}

			// 分割大段落
			subChunks := c.splitLargeParagraph(para, chunkIndex, currentStart)
			chunks = append(chunks, subChunks...)
			chunkIndex += len(subChunks)
			currentStart += paraLen
			continue
		}

		// 检查是否需要开始新分块
		currentLen := utf8.RuneCountInString(currentChunk.String())
		if currentLen+paraLen > c.MaxChunkSize && currentLen >= c.MinChunkSize {
			chunk := c.createChunk(currentChunk.String(), chunkIndex, currentStart)
			chunks = append(chunks, chunk)

			// 保留重叠部分
			overlap := c.getOverlap(currentChunk.String())
			currentChunk.Reset()
			currentChunk.WriteString(overlap)
			currentStart += currentLen - utf8.RuneCountInString(overlap)
			chunkIndex++
		}

		// 添加段落
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)
	}

	// 保存最后一个分块
	if currentChunk.Len() > 0 {
		chunk := c.createChunk(currentChunk.String(), chunkIndex, currentStart)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// ChunkBySection 按章节分块 (适用于小说/文档)
func (c *SemanticChunker) ChunkBySection(content string) ([]*ChunkResult, error) {
	sections := c.splitIntoSections(content)
	if len(sections) == 0 {
		return c.ChunkByParagraph(content)
	}

	var chunks []*ChunkResult
	offset := 0

	for i, section := range sections {
		sectionLen := utf8.RuneCountInString(section.Content)

		if sectionLen <= c.MaxChunkSize {
			// 整个章节作为一个分块
			chunk := c.createChunk(section.Content, i, offset)
			chunk.Metadata = map[string]interface{}{
				"section_title": section.Title,
				"section_level": section.Level,
			}
			chunks = append(chunks, chunk)
		} else {
			// 章节太大，需要进一步分割
			subChunks, _ := c.ChunkByParagraph(section.Content)
			for j, sub := range subChunks {
				sub.ChunkIndex = len(chunks) + j
				sub.Metadata = map[string]interface{}{
					"section_title": section.Title,
					"section_level": section.Level,
				}
			}
			chunks = append(chunks, subChunks...)
		}

		offset += sectionLen
	}

	return chunks, nil
}

// Section 章节结构
type Section struct {
	Title   string
	Content string
	Level   int // 标题级别 (1-6)
}

// splitIntoParagraphs 分割成段落
func (c *SemanticChunker) splitIntoParagraphs(content string) []string {
	// 按空行分割
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(content, -1)

	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}

// splitIntoSections 分割成章节
func (c *SemanticChunker) splitIntoSections(content string) []Section {
	var sections []Section

	// Markdown 标题正则
	headingRegex := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	// 中文章节标题正则
	chineseHeadingRegex := regexp.MustCompile(`(?m)^第[一二三四五六七八九十百千万\d]+[章节回]\s*[：:.]?\s*(.*)$`)

	// 查找所有标题位置
	type heading struct {
		start int
		end   int
		title string
		level int
	}

	var headings []heading

	// 查找 Markdown 标题
	matches := headingRegex.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		level := m[3] - m[2] // # 的数量
		title := content[m[4]:m[5]]
		headings = append(headings, heading{
			start: m[0],
			end:   m[1],
			title: strings.TrimSpace(title),
			level: level,
		})
	}

	// 查找中文章节标题
	chMatches := chineseHeadingRegex.FindAllStringSubmatchIndex(content, -1)
	for _, m := range chMatches {
		title := content[m[0]:m[1]]
		headings = append(headings, heading{
			start: m[0],
			end:   m[1],
			title: strings.TrimSpace(title),
			level: 1,
		})
	}

	// 如果没有找到标题，返回空
	if len(headings) == 0 {
		return nil
	}

	// 按位置排序
	for i := 0; i < len(headings)-1; i++ {
		for j := i + 1; j < len(headings); j++ {
			if headings[i].start > headings[j].start {
				headings[i], headings[j] = headings[j], headings[i]
			}
		}
	}

	// 提取章节内容
	for i, h := range headings {
		var sectionContent string
		if i+1 < len(headings) {
			sectionContent = content[h.end:headings[i+1].start]
		} else {
			sectionContent = content[h.end:]
		}

		sections = append(sections, Section{
			Title:   h.title,
			Content: strings.TrimSpace(sectionContent),
			Level:   h.level,
		})
	}

	return sections
}

// splitLargeParagraph 分割大段落
func (c *SemanticChunker) splitLargeParagraph(para string, startIndex, startOffset int) []*ChunkResult {
	// 尝试按句子分割
	sentences := splitIntoSentences(para)
	if len(sentences) <= 1 {
		// 无法按句子分割，使用固定大小
		return ChunkByFixedSize(para, c.MaxChunkSize, c.OverlapSize)
	}

	var chunks []*ChunkResult
	var current strings.Builder
	chunkIndex := startIndex
	offset := startOffset

	for _, sentence := range sentences {
		sentLen := utf8.RuneCountInString(sentence)
		currentLen := utf8.RuneCountInString(current.String())

		if currentLen+sentLen > c.MaxChunkSize && currentLen >= c.MinChunkSize {
			chunk := c.createChunk(current.String(), chunkIndex, offset)
			chunks = append(chunks, chunk)

			overlap := c.getOverlap(current.String())
			current.Reset()
			current.WriteString(overlap)
			offset += currentLen - utf8.RuneCountInString(overlap)
			chunkIndex++
		}

		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(sentence)
	}

	if current.Len() > 0 {
		chunk := c.createChunk(current.String(), chunkIndex, offset)
		chunks = append(chunks, chunk)
	}

	return chunks
}

// createChunk 创建分块结果
func (c *SemanticChunker) createChunk(content string, index, offset int) *ChunkResult {
	content = strings.TrimSpace(content)
	return &ChunkResult{
		Content:     content,
		ChunkIndex:  index,
		StartOffset: offset,
		EndOffset:   offset + utf8.RuneCountInString(content),
		TokenCount:  estimateTokenCount(content),
		ContentHash: hashContent(content),
	}
}

// getOverlap 获取重叠文本
func (c *SemanticChunker) getOverlap(text string) string {
	if c.OverlapSize <= 0 {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= c.OverlapSize {
		return ""
	}

	overlap := string(runes[len(runes)-c.OverlapSize:])
	// 尝试从句子边界开始
	if idx := strings.LastIndex(overlap, "。"); idx > 0 {
		return overlap[idx+3:] // 跳过句号
	}
	if idx := strings.LastIndex(overlap, ". "); idx > 0 {
		return overlap[idx+2:]
	}
	return overlap
}

// ChunkResult 添加 Metadata 字段
func init() {
	// 确保 ChunkResult 有 Metadata 字段
}
