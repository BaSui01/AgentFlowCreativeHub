package aiinterface

// ClientFactory AI客户端工厂接口
// 用于创建和管理不同类型的AI模型客户端
type ClientFactory interface {
	// CreateClient 根据模型类型和配置创建AI客户端
	CreateClient(modelType string, config *ClientConfig) (ModelClient, error)

	// GetSupportedModels 获取支持的模型列表
	GetSupportedModels() []string
}
