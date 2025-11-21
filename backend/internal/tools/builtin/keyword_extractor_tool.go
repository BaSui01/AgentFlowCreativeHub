package builtin

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"backend/internal/tools"
)

// KeywordExtractorTool 关键词提取工具
type KeywordExtractorTool struct{}

// NewKeywordExtractorTool 创建关键词提取工具
func NewKeywordExtractorTool() *KeywordExtractorTool {
	return &KeywordExtractorTool{}
}

// KeywordScore 关键词及其评分
type KeywordScore struct {
	Word  string
	Score float64
}

// Execute 执行关键词提取
func (t *KeywordExtractorTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 提取参数
	text, ok := input["text"].(string)
	if !ok || text == "" {
		return nil, fmt.Errorf("text 参数类型错误或为空")
	}
	
	topK := 10
	if k, ok := input["top_k"].(float64); ok {
		topK = int(k)
	}
	
	algorithm := "tfidf"
	if algo, ok := input["algorithm"].(string); ok {
		algorithm = algo
	}
	
	language := "auto"
	if lang, ok := input["language"].(string); ok {
		language = lang
	}
	
	// 自动检测语言
	if language == "auto" {
		language = t.detectLanguage(text)
	}
	
	// 提取关键词
	var keywords []KeywordScore
	
	switch algorithm {
	case "tfidf":
		keywords = t.extractByTFIDF(text, language, topK)
	case "frequency":
		keywords = t.extractByFrequency(text, language, topK)
	default:
		keywords = t.extractByTFIDF(text, language, topK)
	}
	
	// 格式化输出
	result := make([]map[string]any, 0, len(keywords))
	for _, kw := range keywords {
		result = append(result, map[string]any{
			"word":  kw.Word,
			"score": fmt.Sprintf("%.3f", kw.Score),
		})
	}
	
	return map[string]any{
		"keywords":       result,
		"algorithm_used": algorithm,
		"total_keywords": len(result),
		"language":       language,
	}, nil
}

// detectLanguage 检测语言
func (t *KeywordExtractorTool) detectLanguage(text string) string {
	chineseCount := 0
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fa5 {
			chineseCount++
		}
	}
	
	if chineseCount > len([]rune(text))/4 {
		return "zh"
	}
	return "en"
}

// extractByTFIDF 使用 TF-IDF 算法提取关键词（基于段落的伪文档方法）
func (t *KeywordExtractorTool) extractByTFIDF(text string, language string, topK int) []KeywordScore {
	// 1. 分割为段落（伪文档）
	paragraphs := t.splitParagraphs(text)
	
	if len(paragraphs) == 0 {
		return []KeywordScore{}
	}
	
	// 2. 计算全局词频 (TF)
	globalTF := make(map[string]float64)
	totalWords := 0
	
	for _, para := range paragraphs {
		words := t.tokenize(para, language)
		for _, word := range words {
			globalTF[word]++
			totalWords++
		}
	}
	
	// 归一化 TF
	for word := range globalTF {
		globalTF[word] = globalTF[word] / float64(totalWords)
	}
	
	// 3. 计算 IDF（基于段落出现频率）
	idf := make(map[string]float64)
	N := float64(len(paragraphs))
	
	for word := range globalTF {
		df := 0.0 // 文档频率（在多少个段落中出现）
		for _, para := range paragraphs {
			if t.containsWord(para, word, language) {
				df++
			}
		}
		if df > 0 {
			idf[word] = math.Log(N / df)
		} else {
			idf[word] = 0
		}
	}
	
	// 4. 计算 TF-IDF
	scores := make([]KeywordScore, 0, len(globalTF))
	for word, tf := range globalTF {
		score := tf * idf[word]
		scores = append(scores, KeywordScore{
			Word:  word,
			Score: score,
		})
	}
	
	// 排序并返回前 K 个
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	
	if len(scores) > topK {
		scores = scores[:topK]
	}
	
	return scores
}

// splitParagraphs 分割段落
func (t *KeywordExtractorTool) splitParagraphs(text string) []string {
	// 统一换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	
	if text == "" {
		return []string{}
	}
	
	// 按空行分割段落
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
	
	// 过滤空段落
	result := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	
	// 如果没有段落分隔，按句子分割
	if len(result) <= 1 && text != "" {
		sentences := regexp.MustCompile(`[。！？.!?]+`).Split(text, -1)
		result = make([]string, 0, len(sentences))
		for _, s := range sentences {
			s = strings.TrimSpace(s)
			if s != "" {
				result = append(result, s)
			}
		}
	}
	
	return result
}

// containsWord 检查段落是否包含指定词
func (t *KeywordExtractorTool) containsWord(paragraph string, word string, language string) bool {
	words := t.tokenize(paragraph, language)
	for _, w := range words {
		if w == word {
			return true
		}
	}
	return false
}

// extractByFrequency 使用词频提取关键词
func (t *KeywordExtractorTool) extractByFrequency(text string, language string, topK int) []KeywordScore {
	// 分词
	words := t.tokenize(text, language)
	
	// 计算词频
	freq := make(map[string]int)
	for _, word := range words {
		freq[word]++
	}
	
	// 转换为 KeywordScore
	scores := make([]KeywordScore, 0, len(freq))
	for word, count := range freq {
		scores = append(scores, KeywordScore{
			Word:  word,
			Score: float64(count),
		})
	}
	
	// 排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	
	if len(scores) > topK {
		scores = scores[:topK]
	}
	
	return scores
}

// tokenize 分词（简化版）
func (t *KeywordExtractorTool) tokenize(text string, language string) []string {
	// 转小写
	text = strings.ToLower(text)
	
	// 移除标点符号
	re := regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	text = re.ReplaceAllString(text, " ")
	
	var words []string
	
	if language == "zh" {
		// 中文分词（简化版：按字符或双字符切分）
		words = t.chineseTokenize(text)
	} else {
		// 英文分词（按空格）
		words = strings.Fields(text)
	}
	
	// 过滤停用词和短词
	filteredWords := make([]string, 0, len(words))
	stopWords := t.getStopWords(language)
	
	for _, word := range words {
		word = strings.TrimSpace(word)
		
		// 跳过空词、单字符词、停用词
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

// chineseTokenize 中文分词（简化版）
func (t *KeywordExtractorTool) chineseTokenize(text string) []string {
	words := make([]string, 0)
	runes := []rune(text)
	
	i := 0
	for i < len(runes) {
		r := runes[i]
		
		// 跳过空格和标点
		if unicode.IsSpace(r) || !unicode.Is(unicode.Han, r) {
			i++
			continue
		}
		
		// 尝试提取 2-4 字的词
		for length := 4; length >= 2; length-- {
			if i+length <= len(runes) {
				word := string(runes[i : i+length])
				// 简单验证：都是汉字
				allHan := true
				for _, wr := range word {
					if !unicode.Is(unicode.Han, wr) {
						allHan = false
						break
					}
				}
				if allHan {
					words = append(words, word)
					i += length
					goto nextWord
				}
			}
		}
		
		// 单字
		if unicode.Is(unicode.Han, r) {
			words = append(words, string(r))
		}
		i++
		
	nextWord:
	}
	
	return words
}

// getStopWords 获取停用词表
func (t *KeywordExtractorTool) getStopWords(language string) map[string]bool {
	stopWords := make(map[string]bool)
	
	if language == "zh" {
		// 中文停用词
		chineseStopWords := []string{
			"的", "了", "是", "在", "我", "有", "和", "就", "不", "人",
			"都", "一", "一个", "上", "也", "很", "到", "说", "要", "去",
			"你", "会", "着", "没有", "看", "好", "自己", "这", "个",
		}
		for _, word := range chineseStopWords {
			stopWords[word] = true
		}
	} else {
		// 英文停用词
		englishStopWords := []string{
			"the", "be", "to", "of", "and", "a", "in", "that", "have", "i",
			"it", "for", "not", "on", "with", "he", "as", "you", "do", "at",
			"this", "but", "his", "by", "from", "they", "we", "say", "her", "she",
			"or", "an", "will", "my", "one", "all", "would", "there", "their",
			"is", "are", "was", "were", "been", "has", "had", "can", "could",
		}
		for _, word := range englishStopWords {
			stopWords[word] = true
		}
	}
	
	return stopWords
}

// Validate 验证输入
func (t *KeywordExtractorTool) Validate(input map[string]any) error {
	text, ok := input["text"].(string)
	if !ok {
		return fmt.Errorf("缺少必需参数: text")
	}
	
	if text == "" {
		return fmt.Errorf("text 参数不能为空")
	}
	
	// 验证算法
	if algorithm, ok := input["algorithm"].(string); ok {
		if algorithm != "" && algorithm != "tfidf" && algorithm != "frequency" {
			return fmt.Errorf("algorithm 必须是 tfidf 或 frequency")
		}
	}
	
	// 验证 top_k
	if topK, ok := input["top_k"].(float64); ok {
		if topK < 1 || topK > 100 {
			return fmt.Errorf("top_k 必须在 1-100 之间")
		}
	}
	
	return nil
}

// GetDefinition 获取工具定义
func (t *KeywordExtractorTool) GetDefinition() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        "keyword_extractor",
		DisplayName: "关键词提取",
		Description: "使用 TF-IDF 或词频算法从文本中提取关键词。支持中英文。适用于自动标签生成、主题分析、SEO 优化等场景。",
		Category:    "text_analysis",
		Type:        "builtin",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "待提取关键词的文本",
				},
				"top_k": map[string]any{
					"type":        "integer",
					"default":     10,
					"minimum":     1,
					"maximum":     100,
					"description": "返回前 K 个关键词",
				},
				"algorithm": map[string]any{
					"type":        "string",
					"enum":        []string{"tfidf", "frequency"},
					"default":     "tfidf",
					"description": "提取算法（tfidf=TF-IDF 算法, frequency=词频统计）",
				},
				"language": map[string]any{
					"type":        "string",
					"enum":        []string{"zh", "en", "auto"},
					"default":     "auto",
					"description": "文本语言",
				},
			},
			"required": []string{"text"},
		},
		Timeout:     10,
		Status:      "active",
		RequireAuth: false,
	}
}
