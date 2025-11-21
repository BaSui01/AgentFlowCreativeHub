# Market Analysis & Requirements Enhancement Spec

## 1. Market Landscape Analysis

Based on the research of similar open-source projects (Dify, Flowise, LangFlow, CrewAI), we have identified the following competitive landscape and feature standards.

### 1.1 Competitor Comparison

| Feature | **AgentFlowCreativeHub** (Goal) | **Dify** | **Flowise** | **CrewAI** |
| :--- | :--- | :--- | :--- | :--- |
| **Core Language** | **Go** (High Perf, Single Binary) | Python/Flask + Next.js | Node.js / TypeScript | Python |
| **Orchestration** | Workflow Engine (State Machine) | Visual + DSL | Visual (Drag & Drop) | Code / Config |
| **Target User** | Enterprise / Content Teams | Developers & Business | Low-code Developers | Python Developers |
| **RAG Pipeline** | Native (Milvus/PgVector) | Advanced (Hybrid, Rerank) | Component-based | External Integration |
| **Deployment** | **Single Binary / Docker** | Docker Compose (Multi-container) | NPM / Docker | Python Library |
| **Visual Builder** | Planned (Phase 3) | ✅ Yes (Workflow Studio) | ✅ Yes (Core Feature) | ❌ No (Code first) |
| **Observability** | Prometheus/Grafana | Built-in Tracing | Limited | ❌ (Requires external) |

### 1.2 Key Differentiators for AgentFlowCreativeHub
1.  **Performance & Simplicity**: being **Go-native** allows for a much lighter footprint and higher concurrency than Python-based alternatives (Dify/LangFlow).
2.  **Deployment**: Single binary deployment is a massive advantage for on-premise enterprise environments compared to the complex microservices of Dify.
3.  **Focus**: "Creative Hub" implies a focus on **Content Production** (Writer/Reviewer agents) rather than just generic "LLM Apps".

---

## 2. Requirements Analysis & Enhancements

Based on the market gaps and project goals, we recommend the following enhancements to the existing `docs/需求分析文档.md`:

### 2.1 Functional Requirements (Enhanced)

#### A. Workflow & Orchestration (Critical)
*   **Current**: "Workflow management, state tracking".
*   **Enhanced**:
    *   **Visual Flow Builder**: Prioritize a visual editor earlier (Phase 2?). Users expect to "see" the agent chain.
    *   **DSL Definition**: Define a YAML/JSON schema for workflows that is portable (export/import).
    *   **Human-in-the-Loop**: Explicit requirement for "Approval Nodes" (essential for Content Creative Hubs).

#### B. Tool & Plugin Ecosystem
*   **Current**: "Agent calling interface".
*   **Enhanced**:
    *   **Standardized Tool Interface**: Define a Go interface `Tool { Name, Description, Schema, Execute }` compatible with OpenAI Function Calling.
    *   **Built-in Tools**: Search (Google/SerpApi), Web Scraper, File Reader.

#### C. RAG Pipeline Specifics
*   **Current**: "Vector retrieval".
*   **Enhanced**:
    *   **Hybrid Search**: Must support Keyword + Vector search (critical for accuracy).
    *   **Reranking**: Add a Rerank step after retrieval for higher quality context.

#### D. AI-Native Observability
*   **Current**: "Logs/Monitoring".
*   **Enhanced**:
    *   **Token & Cost Tracking**: Per-tenant, per-user, per-model cost accounting (Enterprise requirement).
    *   **Traceability**: detailed step-by-step execution logs (Inputs -> Agent -> Tool -> Output) for debugging.

### 2.2 Non-Functional Requirements (Go Specific)

*   **Concurrency**: Explicitly target supporting **10k+ concurrent goroutines** for agent tasks (a key advantage over Python threads).
*   **Startup Time**: < 1s startup time for serverless compatibility.

---

## 3. Proposed Next Steps

1.  **Update Requirements Doc**: Integrate the above enhancements into `docs/需求分析文档.md`.
2.  **Architecture Refinement**: Ensure the Backend Architecture in `backend/` supports the **Standardized Tool Interface** and **Pluggable Orchestrator**.
3.  **Frontend Prototype**: Start designing the **Visual Workflow Builder** UI mocks using React Flow or similar libraries.

*Do you want me to proceed with updating the `docs/需求分析文档.md` with these detailed requirements?*