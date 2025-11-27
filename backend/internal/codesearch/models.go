package codesearch

import "time"

// CodeSymbolType 代码符号类型
type CodeSymbolType string

const (
	SymbolFunction  CodeSymbolType = "function"
	SymbolClass     CodeSymbolType = "class"
	SymbolMethod    CodeSymbolType = "method"
	SymbolVariable  CodeSymbolType = "variable"
	SymbolConstant  CodeSymbolType = "constant"
	SymbolInterface CodeSymbolType = "interface"
	SymbolType      CodeSymbolType = "type"
	SymbolEnum      CodeSymbolType = "enum"
	SymbolImport    CodeSymbolType = "import"
	SymbolExport    CodeSymbolType = "export"
	SymbolStruct    CodeSymbolType = "struct"
)

// CodeSymbol 代码符号
type CodeSymbol struct {
	Name      string         `json:"name"`
	Type      CodeSymbolType `json:"type"`
	FilePath  string         `json:"file_path"`
	Line      int            `json:"line"`
	Column    int            `json:"column"`
	EndLine   int            `json:"end_line,omitempty"`
	EndColumn int            `json:"end_column,omitempty"`
	Signature string         `json:"signature,omitempty"`
	Language  string         `json:"language"`
	Context   string         `json:"context,omitempty"`
}

// CodeReference 代码引用
type CodeReference struct {
	Symbol        string `json:"symbol"`
	FilePath      string `json:"file_path"`
	Line          int    `json:"line"`
	Column        int    `json:"column"`
	Context       string `json:"context"`
	ReferenceType string `json:"reference_type"` // definition, usage, import, type
}

// SearchResult 搜索结果
type SearchResult struct {
	FilePath   string  `json:"file_path"`
	Line       int     `json:"line"`
	Column     int     `json:"column"`
	Content    string  `json:"content"`
	Similarity float64 `json:"similarity,omitempty"`
}

// SemanticSearchResult 语义搜索结果
type SemanticSearchResult struct {
	Query        string          `json:"query"`
	Symbols      []CodeSymbol    `json:"symbols"`
	References   []CodeReference `json:"references"`
	TotalResults int             `json:"total_results"`
	SearchTimeMs int64           `json:"search_time_ms"`
}

// FileOutline 文件大纲
type FileOutline struct {
	FilePath string       `json:"file_path"`
	Language string       `json:"language"`
	Symbols  []CodeSymbol `json:"symbols"`
}

// CodeChunk 代码块（用于语义搜索）
type CodeChunk struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	Embedding []float32 `json:"embedding,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TextSearchOptions 文本搜索选项
type TextSearchOptions struct {
	Pattern       string `json:"pattern"`
	FileGlob      string `json:"file_glob,omitempty"`
	IsRegex       bool   `json:"is_regex"`
	MaxResults    int    `json:"max_results"`
	CaseSensitive bool   `json:"case_sensitive"`
}

// SymbolSearchOptions 符号搜索选项
type SymbolSearchOptions struct {
	Query      string         `json:"query"`
	SymbolType CodeSymbolType `json:"symbol_type,omitempty"`
	Language   string         `json:"language,omitempty"`
	MaxResults int            `json:"max_results"`
}
