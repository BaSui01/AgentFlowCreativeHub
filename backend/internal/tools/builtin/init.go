package builtin

import (
	"backend/internal/tools"
)

// RegisterAll 注册所有内置工具
func RegisterAll(registry *tools.ToolRegistry) error {
	// 通用工具
	calculator := NewCalculatorTool()
	if err := registry.Register(calculator.GetDefinition().Name, calculator, calculator.GetDefinition()); err != nil {
		return err
	}
	
	knowledge := NewKnowledgeTool(nil) // KBService 会在调用时注入
	if err := registry.Register(knowledge.GetDefinition().Name, knowledge, knowledge.GetDefinition()); err != nil {
		return err
	}
	
	search := NewSearchTool()
	if err := registry.Register(search.GetDefinition().Name, search, search.GetDefinition()); err != nil {
		return err
	}
	
	httpAPI := NewHTTPAPITool()
	if err := registry.Register(httpAPI.GetDefinition().Name, httpAPI, httpAPI.GetDefinition()); err != nil {
		return err
	}
	
	// 内容创作专用工具
	textStats := NewTextStatisticsTool()
	if err := registry.Register(textStats.GetDefinition().Name, textStats, textStats.GetDefinition()); err != nil {
		return err
	}
	
	textConverter := NewTextConverterTool()
	if err := registry.Register(textConverter.GetDefinition().Name, textConverter, textConverter.GetDefinition()); err != nil {
		return err
	}
	
	keywordExtractor := NewKeywordExtractorTool()
	if err := registry.Register(keywordExtractor.GetDefinition().Name, keywordExtractor, keywordExtractor.GetDefinition()); err != nil {
		return err
	}
	
	textSummarizer := NewTextSummarizerTool()
	if err := registry.Register(textSummarizer.GetDefinition().Name, textSummarizer, textSummarizer.GetDefinition()); err != nil {
		return err
	}
	
	return nil
}
