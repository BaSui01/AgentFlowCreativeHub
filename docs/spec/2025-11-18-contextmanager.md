## 背景
`internal/agent/runtime/context_manager_test.go` 仍然依赖旧版 `ContextManager` API（如无 `context.Context` 参数的 `CreateSession`、`AddMessage`，以及已删除的 TTL 相关方法），导致当前 `go test ./...` 编译失败。

## 主要问题
1. `NewContextManager`、`CreateSession`、`AddMessage`、`SetData`、`EnrichInput` 等调用签名与实现不匹配。
2. 旧测试依赖不存在的方法（`GetSessionCount`、`ClearExpiredSessions` 等），无法继续使用。
3. 未对 `SessionStore` 进行可控替身，难以精准验证行为。

## 调整方案
1. **测试基建改造**
   - 新增 `mockSessionStore`（实现 `SessionStore` 接口，基于 map 存储），用于捕获保存/获取/删除行为，便于断言。
   - 新增 `newTestContextManager()` 帮助函数（内部使用 mock store 并返回 `ContextManager` 与 store），复用在各测试中。

2. **重写/新增测试用例**
   - `TestCreateAndGetSession`: 调用 `CreateSession(ctx, tenant, user, session)` 后使用 `GetSession` 验证存储字段与时间戳更新逻辑。
   - `TestAddMessageAndHistoryLimit`: 通过 `AddMessage` 写入多条记录，调用 `EnrichInput(ctx, input, sessionID, 3, 0, "gpt-3.5-turbo")` 验证仅返回最近 3 条并保持顺序。
   - `TestEnrichInputSummaryMode`: 先 `SetData` 写入 `memorySummaryKey`，配置 `input.ExtraParams["memory_mode"] = memoryModeSummary`，确认返回的第一条消息是系统摘要，其余为最近历史。
   - `TestAddMessageMissingSession`: 不创建会话直接 `AddMessage`，断言返回 `ErrSessionNotFound`，覆盖异常路径。
   - 若需要额外覆盖，可加 `TestSetAndGetData` 保障键值存取逻辑。
   - 删除旧的 TTL 相关测试。

3. **实现细节**
   - 所有测试调用 `context.Background()` 传入 `ContextManager` 方法，确保与新签名一致。
   - 使用 `t.Helper()` 的 mock store 断言失败时输出更易读。
   - 充分利用包级常量 `memoryModeSummary`/`memorySummaryKey`，避免硬编码字符串。

4. **验证**
   - 更新测试后执行 `go test ./...`，确认全部通过并恢复基线验证。

如需额外覆盖（例如 `EnrichInput` 在会话缺失时静默返回）可在上述基础上再补充一条轻量测试。