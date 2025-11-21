package rag

import "context"

// EmbeddingProvider 抽象不同向量模型/服务的统一接口。
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	GetModel() string
	GetProviderName() string
}

// EmbeddingModelConfig 描述一个嵌入模型的关键配置，用于上层做路由和参数校验。
type EmbeddingModelConfig struct {
	Name       string
	Dimensions int
	MaxBatch   int
}
