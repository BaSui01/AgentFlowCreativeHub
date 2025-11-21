## 问题
虽然 `loadEnvFile()` 已增加多级 fallback，但运行在更深层目录或被其他进程（例如 `asynq` worker）启动时仍可能找不到根目录 `.env`，导致 Redis 依旧指向 localhost 并疯狂报错。

## 方案
1. **统一定位仓库根目录**
   - 使用 `os.Getwd()` 获取当前工作目录，然后沿父级目录向上遍历（最多 5 层），寻找包含 `go.mod` 的目录（即 backend 模块根）再拼接 `../.env`（项目根）。
   - 或者利用 `filepath.Abs(os.Args[0])`（可执行所在目录）推导出 `backend/cmd/server`，从而计算项目根。

2. **只加载根目录 .env**
   - 找到根目录路径后，直接 `godotenv.Load(filepath.Join(projectRoot, ".env"))`。
   - 若加载失败则打印明确日志并继续（避免 panic），但不再尝试其它相对路径，确保环境来源唯一。

3. **保持兼容**
   - 若 `.env` 不存在，维持现有行为（配置来自 `config/dev.yaml` + APP_* 环境），但记录一条警告方便排查。

4. **验证**
   - 在 `backend`、`backend/cmd/server` 和项目根目录分别执行 `go run ./cmd/server`，确保均能读取根目录 `.env` 并输出远程 Redis/DB 连接信息。