package builtin

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"backend/internal/tools"
)

// TextSummarizerTool 文本摘要工具
type TextSummarizerTool struct{}

// NewTextSummarizerTool 创建文本摘要工具
func NewTextSummarizerTool() *TextSummarizerTool {
	return &TextSummarizerTool{}
}

// SentenceScore 句子及其评分
type SentenceScore struct {
	Sentence string
	Score    float64
	Position int
}

// Execute 执行文本摘要
func (t *TextSummarizerTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 提取参数
	text, ok := input["text"].(string)
	if !ok || text == "" {
		return nil, fmt.Errorf("text 参数类型错误或为空")
	}
	
	method := "extractive"
	if m, ok := input["method"].(string); ok {
		method = m
	}
	
	maxSentences := 3
	if ms, ok := input["max_sentences"].(float64); ok {
		maxSentences = int(ms)
	}
	
	maxLength := 200
	if ml, ok := input["max_length"].(float64); ok {
		maxLength = int(ml)
	}
	
	// 执行摘要
	var summary string
	var sentencesUsed int
	
	switch method {
	case "extractive":
		summary, sentencesUsed = t.extractiveSummary(text, maxSentences, maxLength)
	default:
		summary, sentencesUsed = t.extractiveSummary(text, maxSentences, maxLength)
	}
	
	return map[string]any{
		"summary":        summary,
		"method":         method,
		"sentences_used": sentencesUsed,
		"total_length":   len([]rune(summary)),
	}, nil
}

// extractiveSummary 提取式摘要
func (t *TextSummarizerTool) extractiveSummary(text string, maxSentences int, maxLength int) (string, int) {
	// 分句
	sentences := t.splitSentences(text)
	
	if len(sentences) == 0 {
		return "", 0
	}
	
	// 如果句子数少于要求，返回全部
	if len(sentences) <= maxSentences {
		summary := strings.Join(sentences, "")
		return summary, len(sentences)
	}
	
	// 计算每个句子的重要性评分
	sentenceScores := t.scoreSentences(sentences)
	
	// 按评分排序
	sort.Slice(sentenceScores, func(i, j int) bool {
		return sentenceScores[i].Score > sentenceScores[j].Score
	})
	
	// 选择前 N 个重要句子
	selected := sentenceScores[:maxSentences]
	
	// 按原文顺序重新排序
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Position < selected[j].Position
	})
	
	// 生成摘要
	var summaryParts []string
	currentLength := 0
	sentencesUsed := 0
	
	for _, s := range selected {
		sentenceLength := len([]rune(s.Sentence))
		if currentLength+sentenceLength > maxLength && sentencesUsed > 0 {
			break
		}
		summaryParts = append(summaryParts, s.Sentence)
		currentLength += sentenceLength
		sentencesUsed++
	}
	
	summary := strings.Join(summaryParts, "")
	return summary, sentencesUsed
}

// splitSentences 分句
func (t *TextSummarizerTool) splitSentences(text string) []string {
	// 统一换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	
	if text == "" {
		return []string{}
	}
	
	// 按句子结束符分割
	// 中文：。！？
	// 英文：.!?
	re := regexp.MustCompile(`[。！？.!?]+`)
	parts := re.Split(text, -1)
	
	sentences := make([]string, 0, len(parts))
	
	// 处理分割后的句子片段
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// 过滤太短的句子
		if len([]rune(part)) <= 5 {
			continue
		}
		
		// 如果不是最后一个片段，恢复句尾标点
		// 通过在原文中查找对应位置的标点
		if i < len(parts)-1 {
			// 在下一个片段前找到的标点就是当前句子的结尾
			sentences = append(sentences, part)
		} else {
			// 最后一个片段，检查原文是否有标点结尾
			if re.MatchString(text) {
				// 如果原文有标点，这个片段应该已经包含了
				sentences = append(sentences, part)
			} else {
				// 原文没有标点结尾，但仍然是一个句子
				sentences = append(sentences, part)
			}
		}
	}
	
	return sentences
}

// scoreSentences 计算句子评分
func (t *TextSummarizerTool) scoreSentences(sentences []string) []SentenceScore {
	scores := make([]SentenceScore, 0, len(sentences))
	
	// 计算全局词频
	wordFreq := make(map[string]int)
	for _, sentence := range sentences {
		words := t.tokenize(sentence)
		for _, word := range words {
			wordFreq[word]++
		}
	}
	
	// 计算每个句子的评分
	for i, sentence := range sentences {
		score := t.scoreSentence(sentence, wordFreq, i, len(sentences))
		scores = append(scores, SentenceScore{
			Sentence: sentence,
			Score:    score,
			Position: i,
		})
	}
	
	return scores
}

// scoreSentence 计算单个句子的评分
func (t *TextSummarizerTool) scoreSentence(sentence string, wordFreq map[string]int, position int, totalSentences int) float64 {
	words := t.tokenize(sentence)
	
	if len(words) == 0 {
		return 0.0
	}
	
	// 1. 词频评分
	freqScore := 0.0
	for _, word := range words {
		freqScore += float64(wordFreq[word])
	}
	freqScore = freqScore / float64(len(words))
	
	// 2. 位置评分（开头和结尾的句子更重要）
	positionScore := 1.0
	if position == 0 {
		positionScore = 2.0 // 第一句很重要
	} else if position == totalSentences-1 {
		positionScore = 1.5 // 最后一句也重要
	} else if position < totalSentences/4 {
		positionScore = 1.3 // 前四分之一比较重要
	}
	
	// 3. 句子长度评分（中等长度的句子更好）
	sentenceLength := len([]rune(sentence))
	lengthScore := 1.0
	if sentenceLength >= 20 && sentenceLength <= 100 {
		lengthScore = 1.2
	} else if sentenceLength < 10 || sentenceLength > 200 {
		lengthScore = 0.5
	}
	
	// 综合评分
	totalScore := freqScore * positionScore * lengthScore
	
	return totalScore
}

// tokenize 简单分词
func (t *TextSummarizerTool) tokenize(text string) []string {
	// 转小写
	text = strings.ToLower(text)
	
	// 移除标点
	re := regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	text = re.ReplaceAllString(text, " ")
	
	// 分词
	words := strings.Fields(text)
	
	// 过滤停用词和短词
	filteredWords := make([]string, 0, len(words))
	stopWords := t.getStopWords()
	
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" || len([]rune(word)) < 2 {
			continue
		}
		if _, isStopWord := stopWords[word]; isStopWord {
			continue
		}
		filteredWords = append(filteredWords, word)
	}
	
	return filteredWords
}

// getStopWords 获取停用词
func (t *TextSummarizerTool) getStopWords() map[string]bool {
	stopWords := make(map[string]bool)
	
	// 中英文常见停用词
	commonStopWords := []string{
		// 中文
		"的", "了", "是", "在", "我", "有", "和", "就", "不", "人",
		"都", "一", "一个", "上", "也", "很", "到", "说", "要", "去",
		// 英文
		"the", "be", "to", "of", "and", "a", "in", "that", "have", "i",
		"it", "for", "not", "on", "with", "he", "as", "you", "do", "at",
		"is", "are", "was", "were", "been", "has", "had",
	}
	
	for _, word := range commonStopWords {
		stopWords[word] = true
	}
	
	return stopWords
}

// Validate 验证输入
func (t *TextSummarizerTool) Validate(input map[string]any) error {
	text, ok := input["text"].(string)
	if !ok {
		return fmt.Errorf("缺少必需参数: text")
	}
	
	if text == "" {
		return fmt.Errorf("text 参数不能为空")
	}
	
	// 验证方法
	if method, ok := input["method"].(string); ok {
		if method != "" && method != "extractive" && method != "llm" {
			return fmt.Errorf("method 必须是 extractive 或 llm")
		}
	}
	
	// 验证 max_sentences
	if maxSentences, ok := input["max_sentences"].(float64); ok {
		if maxSentences < 1 || maxSentences > 20 {
			return fmt.Errorf("max_sentences 必须在 1-20 之间")
		}
	}
	
	// 验证 max_length
	if maxLength, ok := input["max_length"].(float64); ok {
		if maxLength < 50 || maxLength > 5000 {
			return fmt.Errorf("max_length 必须在 50-5000 之间")
		}
	}
	
	return nil
}

// GetDefinition 获取工具定义
func (t *TextSummarizerTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "text_summarizer",
		DisplayName: "文本摘要",
		Description: "从文本中提取关键句子生成摘要。使用提取式算法选择最重要的句子。支持指定摘要长度和句子数。",
		Category:    "text_analysis",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "待摘要的文本",
				},
				"method": map[string]any{
					"type":        "string",
					"enum":        []string{"extractive", "llm"},
					"default":     "extractive",
					"description": "摘要方法（extractive=提取式, llm=生成式）",
				},
				"max_sentences": map[string]any{
					"type":        "integer",
					"default":     3,
					"minimum":     1,
					"maximum":     20,
					"description": "最大句子数",
				},
				"max_length": map[string]any{
					"type":        "integer",
					"default":     200,
					"minimum":     50,
					"maximum":     5000,
					"description": "最大字数",
				},
			},
			"required": []string{"text"},
		},
		Timeout:     10,
		Status:      "active",
		RequireAuth: false,
	}
}
