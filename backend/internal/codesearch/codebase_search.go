package codesearch

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backend/internal/logger"
	"backend/internal/rag"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CodebaseSearchService 代码库语义搜索服务
// 使用 embedding 进行语义搜索
type CodebaseSearchService struct {
	basePath      string
	embeddings    rag.EmbeddingProvider
	chunks        []CodeChunk
	chunkSize     int
	chunkOverlap  int
	mu            sync.RWMutex
	lastIndexTime time.Time
	logger        *zap.Logger
}

// NewCodebaseSearchService 创建代码库搜索服务
func NewCodebaseSearchService(basePath string, embeddings rag.EmbeddingProvider) *CodebaseSearchService {
	return &CodebaseSearchService{
		basePath:     basePath,
		embeddings:   embeddings,
		chunks:       make([]CodeChunk, 0),
		chunkSize:    100, // 每个块的行数
		chunkOverlap: 20,  // 重叠行数
		logger:       logger.Get(),
	}
}

// BuildIndex 构建代码库索引
func (s *CodebaseSearchService) BuildIndex(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chunks = make([]CodeChunk, 0)
	excludeDirs := map[string]bool{
		"node_modules": true, ".git": true, "dist": true,
		"build": true, "__pycache__": true, "vendor": true,
		".idea": true, "target": true, "coverage": true,
	}

	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			if excludeDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		lang := detectLanguage(path)
		if lang == "" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(s.basePath, path)
		lines := strings.Split(string(content), "\n")
		fileChunks := s.chunkFile(relPath, lines, lang)
		s.chunks = append(s.chunks, fileChunks...)

		return nil
	})

	if err != nil {
		return err
	}

	// 生成 embeddings
	if s.embeddings != nil {
		for i := range s.chunks {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			embedding, err := s.embeddings.Embed(ctx, s.chunks[i].Content)
			if err != nil {
				s.logger.Debug("生成 embedding 失败", zap.String("chunk_id", s.chunks[i].ID), zap.Error(err))
				continue
			}
			s.chunks[i].Embedding = embedding
		}
	}

	s.lastIndexTime = time.Now()
	s.logger.Info("代码库索引构建完成", zap.Int("chunks", len(s.chunks)))
	return nil
}

// chunkFile 将文件分割成块
func (s *CodebaseSearchService) chunkFile(filePath string, lines []string, language string) []CodeChunk {
	chunks := make([]CodeChunk, 0)

	for start := 0; start < len(lines); start += s.chunkSize - s.chunkOverlap {
		end := start + s.chunkSize
		if end > len(lines) {
			end = len(lines)
		}

		content := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(content) == "" {
			continue
		}

		chunks = append(chunks, CodeChunk{
			ID:        uuid.New().String(),
			FilePath:  filePath,
			StartLine: start + 1,
			EndLine:   end,
			Content:   content,
			Language:  language,
			CreatedAt: time.Now(),
		})

		if end >= len(lines) {
			break
		}
	}

	return chunks
}

// Search 语义搜索
func (s *CodebaseSearchService) Search(ctx context.Context, query string, topN int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.chunks) == 0 {
		return nil, fmt.Errorf("代码库索引为空，请先运行索引构建")
	}

	if topN <= 0 {
		topN = 10
	}

	// 生成查询 embedding
	var queryEmbedding []float32
	if s.embeddings != nil {
		var err error
		queryEmbedding, err = s.embeddings.Embed(ctx, query)
		if err != nil {
			s.logger.Warn("生成查询 embedding 失败，回退到关键词搜索", zap.Error(err))
		}
	}

	// 计算相似度并排序
	type scoredChunk struct {
		chunk      CodeChunk
		similarity float64
	}
	scored := make([]scoredChunk, 0, len(s.chunks))

	for _, chunk := range s.chunks {
		var similarity float64

		if queryEmbedding != nil && len(chunk.Embedding) > 0 {
			// 使用余弦相似度
			similarity = cosineSimilarity(queryEmbedding, chunk.Embedding)
		} else {
			// 回退到关键词匹配
			similarity = keywordSimilarity(query, chunk.Content)
		}

		if similarity > 0 {
			scored = append(scored, scoredChunk{chunk: chunk, similarity: similarity})
		}
	}

	// 排序
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].similarity > scored[i].similarity {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// 取前 N 个
	results := make([]SearchResult, 0, topN)
	for i := 0; i < len(scored) && i < topN; i++ {
		chunk := scored[i].chunk
		results = append(results, SearchResult{
			FilePath:   chunk.FilePath,
			Line:       chunk.StartLine,
			Content:    chunk.Content,
			Similarity: scored[i].similarity,
		})
	}

	return results, nil
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// keywordSimilarity 关键词相似度（回退方案）
func keywordSimilarity(query, content string) float64 {
	queryLower := strings.ToLower(query)
	contentLower := strings.ToLower(content)

	words := strings.Fields(queryLower)
	if len(words) == 0 {
		return 0
	}

	matchCount := 0
	for _, word := range words {
		if strings.Contains(contentLower, word) {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(words))
}

// GetTotalChunks 获取总块数
func (s *CodebaseSearchService) GetTotalChunks() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks)
}

// GetLastIndexTime 获取最后索引时间
func (s *CodebaseSearchService) GetLastIndexTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastIndexTime
}

// SetBasePath 设置基础路径
func (s *CodebaseSearchService) SetBasePath(basePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.basePath = basePath
	s.chunks = make([]CodeChunk, 0)
	s.lastIndexTime = time.Time{}
}

// IsIndexReady 检查索引是否就绪
func (s *CodebaseSearchService) IsIndexReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks) > 0
}
