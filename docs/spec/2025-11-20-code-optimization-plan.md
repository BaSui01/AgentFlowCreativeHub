# 2025-11-20 优化完善计划

## 背景
- 聚焦后端启动路径和监控中间件的安全/性能小缺口，确保生产安全门槛和基础指标采集健壮性。

## 计划任务
| 序号 | 任务 | 目标/衡量 | 状态 | 备注 |
| --- | --- | --- | --- | --- |
| 1 | JWT 密钥加载收紧 | 生产模式下缺失 `JWT_SECRET_KEY` 时阻断启动；开发模式仅提示警告 | 已完成 | backend/api/setup.go（release/prod 缺密钥直接 Fatal；dev 输出警告） |
| 2 | Prometheus 中间件响应体统计优化 | 移除响应体拷贝，使用 `Writer.Size()` 采集，兼容流式/大响应 | 已完成 | backend/internal/metrics/middleware.go（去除响应缓冲，使用 Writer.Size()） |
| 3 | 数据库连接错误处理加固 | 获取底层 `sql.DB` 失败立即中止，避免返回空路由 | 已完成 | backend/api/setup.go（获取 sql.DB 失败时立即 Fatal 退出） |

## 验收与更新
- 2025-11-20：3/3 任务已完成；`gofmt` 已执行。尝试 `go test ./api ./internal/metrics` 时命令在默认超时内未返回，尚未拿到测试结果（可能因拉取依赖耗时）；后续如需可单独延长超时重跑。
