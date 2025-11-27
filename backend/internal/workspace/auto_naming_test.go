package workspace

import (
	"strings"
	"testing"
)

func TestGenerateArtifactPath(t *testing.T) {
	policy := NewAutoNamingPolicy(
		WithOrganizeByAgent(true),
		WithOrganizeBySession(false),
	)

	tests := []struct {
		name     string
		req      *ArtifactNamingRequest
		wantDir  string
		wantExt  string
	}{
		{
			name: "智能体产出大纲",
			req: &ArtifactNamingRequest{
				AgentName: "planner",
				AgentID:   "agent-001",
				TaskType:  ArtifactTypeOutline,
				TitleHint: "项目规划",
			},
			wantDir: "agents/planner/outputs",
			wantExt: ".md",
		},
		{
			name: "智能体产出代码",
			req: &ArtifactNamingRequest{
				AgentName: "writer",
				AgentID:   "agent-002",
				TaskType:  ArtifactTypeCode,
				Content:   "package main\n\nfunc main() {}",
			},
			wantDir: "agents/writer/outputs",
			wantExt: ".go",
		},
		{
			name: "智能体产出数据",
			req: &ArtifactNamingRequest{
				AgentName: "analyzer",
				AgentID:   "agent-003",
				TaskType:  ArtifactTypeData,
				Content:   `{"key": "value"}`,
			},
			wantDir: "agents/analyzer/outputs",
			wantExt: ".json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.GenerateArtifactPath(tt.req)

			if result.FolderPath != tt.wantDir {
				t.Errorf("FolderPath = %q, want %q", result.FolderPath, tt.wantDir)
			}

			if !strings.HasSuffix(result.FileName, tt.wantExt) {
				t.Errorf("FileName = %q, want suffix %q", result.FileName, tt.wantExt)
			}

			if result.FullPath == "" {
				t.Error("FullPath should not be empty")
			}

			// 验证路径格式
			if !strings.HasPrefix(result.FullPath, result.FolderPath) {
				t.Errorf("FullPath should start with FolderPath")
			}
		})
	}
}

func TestGenerateArtifactPathWithSession(t *testing.T) {
	policy := NewAutoNamingPolicy(
		WithOrganizeByAgent(false),
		WithOrganizeBySession(true),
	)

	req := &ArtifactNamingRequest{
		AgentName: "planner",
		AgentID:   "agent-001",
		SessionID: "session-abc-123",
		TaskType:  ArtifactTypeOutline,
	}

	result := policy.GenerateArtifactPath(req)

	if !strings.HasPrefix(result.FolderPath, "sessions/") {
		t.Errorf("FolderPath should start with 'sessions/', got %q", result.FolderPath)
	}

	if result.SessionFolder == "" {
		t.Error("SessionFolder should not be empty")
	}
}

func TestSlugifyAgent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Planner", "planner"},
		{"Researcher", "researcher"},
		{"My Custom Agent", "my-custom-agent"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugifyAgent(tt.input)
			if got != tt.want {
				t.Errorf("slugifyAgent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInferExtension(t *testing.T) {
	tests := []struct {
		artifactType ArtifactType
		content      string
		want         string
	}{
		{ArtifactTypeCode, "package main\n\nfunc main() {}", ".go"},
		{ArtifactTypeCode, "def hello():\n    pass", ".py"},
		{ArtifactTypeCode, "function test() {}", ".js"},
		{ArtifactTypeData, `{"key": "value"}`, ".json"},
		{ArtifactTypeData, "a,b,c\n1,2,3", ".csv"},
		{ArtifactTypeOutline, "# Title", ".md"},
		{ArtifactTypeReport, "Report content", ".md"},
	}

	for _, tt := range tests {
		t.Run(string(tt.artifactType), func(t *testing.T) {
			got := inferExtension(tt.artifactType, tt.content)
			if got != tt.want {
				t.Errorf("inferExtension(%q, ...) = %q, want %q", tt.artifactType, got, tt.want)
			}
		})
	}
}

func TestSequenceCounter(t *testing.T) {
	policy := NewAutoNamingPolicy()

	// 第一次调用应该返回 1
	seq1 := policy.getNextSequence("session-1", "agent-1")
	if seq1 != 1 {
		t.Errorf("First sequence should be 1, got %d", seq1)
	}

	// 第二次调用应该返回 2
	seq2 := policy.getNextSequence("session-1", "agent-1")
	if seq2 != 2 {
		t.Errorf("Second sequence should be 2, got %d", seq2)
	}

	// 不同会话应该重新开始
	seq3 := policy.getNextSequence("session-2", "agent-1")
	if seq3 != 1 {
		t.Errorf("Different session should start at 1, got %d", seq3)
	}

	// 重置后应该从 1 开始
	policy.ResetSequence("session-1", "agent-1")
	seq4 := policy.getNextSequence("session-1", "agent-1")
	if seq4 != 1 {
		t.Errorf("After reset, sequence should be 1, got %d", seq4)
	}
}
