1. 说明 sequential-thinking 工具所在的 MCP 服务器：根据当前 `mcp.json`，该工具由 `mcphub` 服务器提供，因此调用前需确认 `mcphub` 条目里的 URL 与 `Authorization` token 已正确配置，并确保网络可访问。
2. 给出调用命令格式：在 CLI 中运行 `mcp__mcphub__sequential-thinking`（或在 UI 中选择同名工具），若提示确认，选择 “Yes, allow” 继续。
3. 阐明必要输入：根据任务描述提供 `query` 或上下文参数（若该工具需要），并说明高影响 MCP 调用仍会弹出确认，无法跳过；完成后可重复调用或按需输入不同问题。