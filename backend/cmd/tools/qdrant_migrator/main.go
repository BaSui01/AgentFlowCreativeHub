package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"backend/internal/config"
	"backend/internal/infra"
	"backend/internal/rag"
)

func main() {
	env := flag.String("env", "dev", "配置环境 dev/prod/test")
	batchSize := flag.Int("batch", 200, "每批迁移的向量数量")
	dryRun := flag.Bool("dry-run", false, "仅打印不写入 Qdrant")
	flag.Parse()

	cfg, err := config.Load(*env, "")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	db, err := infra.InitDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer infra.CloseDatabase()

	store, err := rag.NewQdrantStore(rag.QdrantOptions{
		Endpoint:        cfg.RAG.VectorStore.Qdrant.Endpoint,
		APIKey:          cfg.RAG.VectorStore.Qdrant.APIKey,
		Collection:      cfg.RAG.VectorStore.Qdrant.Collection,
		VectorDimension: cfg.RAG.VectorStore.Qdrant.VectorDimension,
		Distance:        cfg.RAG.VectorStore.Qdrant.Distance,
		TimeoutSeconds:  cfg.RAG.VectorStore.Qdrant.TimeoutSeconds,
	})
	if err != nil {
		log.Fatalf("初始化 Qdrant 失败: %v", err)
	}

	ctx := context.Background()
	totalMigrated := 0
	for {
		var chunks []rag.KnowledgeChunk
		if err := db.WithContext(ctx).
			Where("deleted_at IS NULL").
			Order("created_at ASC").
			Limit(*batchSize).
			Offset(totalMigrated).
			Find(&chunks).Error; err != nil {
			log.Fatalf("查询 knowledge_chunks 失败: %v", err)
		}

		if len(chunks) == 0 {
			break
		}

		vectors := make([]*rag.Vector, 0, len(chunks))
		for _, chunk := range chunks {
			vec, err := chunkToVector(&chunk)
			if err != nil {
				log.Printf("跳过 chunk %s: %v", chunk.ID, err)
				continue
			}
			vectors = append(vectors, vec)
		}

		if *dryRun {
			fmt.Printf("[dry-run] 计划迁移 %d 条向量\n", len(vectors))
		} else {
			if err := store.AddVectors(ctx, vectors); err != nil {
				log.Fatalf("写入 Qdrant 失败: %v", err)
			}
		}

		totalMigrated += len(chunks)
		fmt.Printf("已处理 %d 条向量\n", totalMigrated)
	}

	fmt.Printf("迁移完成，总计 %d 条向量\n", totalMigrated)
}

func chunkToVector(chunk *rag.KnowledgeChunk) (*rag.Vector, error) {
	embedding, err := parseVectorString(chunk.Embedding)
	if err != nil {
		return nil, err
	}
	return &rag.Vector{
		ChunkID:           chunk.ID,
		KnowledgeBaseID:   chunk.KnowledgeBaseID,
		DocumentID:        chunk.DocumentID,
		Content:           chunk.Content,
		ContentHash:       chunk.ContentHash,
		ChunkIndex:        chunk.ChunkIndex,
		StartOffset:       chunk.StartOffset,
		EndOffset:         chunk.EndOffset,
		TokenCount:        chunk.TokenCount,
		Embedding:         embedding,
		EmbeddingModel:    chunk.EmbeddingModel,
		EmbeddingProvider: chunk.EmbeddingProvider,
		Metadata:          chunk.MetadataRaw,
	}, nil
}

func parseVectorString(value string) ([]float32, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if value == "" {
		return nil, fmt.Errorf("向量数据为空")
	}
	parts := strings.Split(value, ",")
	vec := make([]float32, 0, len(parts))
	for _, part := range parts {
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%f", &f); err != nil {
			return nil, fmt.Errorf("解析向量失败: %w", err)
		}
		vec = append(vec, float32(f))
	}
	return vec, nil
}
