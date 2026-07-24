# AI_RAG_DEMO AI 编码指导

## 核心定位与导航 (gopls 逻辑)

1. **精确搜索**: 优先使用gopls指令，然后使用符号搜索而非全文搜索。定位业务逻辑时，搜索 `type [Name]Biz struct` 或 `type [Name]Repo struct`。
2. **架构映射**: 
   - `api/` -> 定义层 (Proto 文件, 错误码定义)
   - `cmd/server/` -> 程序入口 (main.go, wire.go 注入点)
   - `configs/` -> 配置文件 (YAML, Nacos 配置)
   - `internal/service/` -> 协议实现层 (实现 Proto 接口, 参数 DTO 转换)
   - `internal/biz/` -> 核心业务逻辑层 (领域编排, **禁跨包调用**)
   - `internal/biz/common/` -> 共享业务逻辑 (跨模块复用代码存放点)
   - `internal/data/` -> 数据持久化层 (Repository 实现, DB CRUD)
   - `internal/cache/` -> 缓存层 (Redis 实现, 分布式锁 `redis_lock.go`)
   - `internal/conf/` -> 配置结构定义 (Go Structs 映射 YAML)
   - `internal/external/` -> 外部服务集成 (阿里云 OSS, IoT 平台客户端, 短信/微信 SDK)
   - `internal/pkg/` -> 项目公共组件 (日志 `log`, 国际化 `i18n`, 工具类 `utils`)
   - `internal/server/` -> 通信协议服务 (HTTP/gRPC Server 初始化)
   - `sqls/` -> 数据库变更脚本 (SQL 迁移)

## gopls 完整命令参考 (AI 提效工具)

使用格式: `gopls [command] [args...]`。在定位和分析时，优先通过 `run_shell_command` 调用 gopls。

### 1. 导航与搜索 (Navigation)
- `definition [target]`: 跳转到声明位置。`target` 格式为 `file:line:col`。
- `implementation [target]`: 查找接口的实现。
- `references [target]`: 查找所有引用点。
- `symbols [file]`: 提取单个文件内的所有符号。
- `workspace_symbol [query]`: 全局模糊搜索符号。
- `call_hierarchy [target]`: 显示调用层级。

### 2. 代码分析 (Analysis)
- `check [file]`: 诊断文件错误（语法、类型、未使用变量）。
- `signature [target]`: 显示函数签名信息。
- `highlight [target]`: 高亮当前文件内的相同标识符。
- `stats`: 输出当前工作区的统计数据。

### 3. 代码操作 (Manipulation)
- `format [file]`: 格式化代码。
- `imports [file]`: 修复导入语句（自动增删）。
- `rename [target] [new_name]`: 全局重命名标识符。
- `prepare_rename [target]`: 检查重命名合法性。
- `codeaction [file]`: 列出或执行代码建议操作。
- `fix [file]`: 应用建议的修复（旧版）。

### 4. 高级功能
- `folding_ranges [file]`: 显示代码折叠区间。
- `links [file]`: 列出文件中的链接。
- `semtok [file]`: 显示语义标记（用于着色分析）。
- `codelens [file]`: 列出或执行代码透镜操作。

## 核心开发禁令 (Mandates)

### 1. 跨 Biz 调用禁令
- **绝对禁止**: 跨 `internal/biz/` 下的子包直接调用（如 `iot` 包禁止直接调用 `ai-rag-demo` 包中的 Biz）。
- **共享逻辑**: 必须将共享逻辑提取到 `internal/biz/common/`（如 `ai-rag-demo_sub.go`）。
- **参数传递**: `common` 包中的共享方法直接使用 proto 生成的 Req/Resp 类型，避免重复定义。

### 2. 安全协程禁令
- **绝对禁止**: 直接使用原生的 `go func()`。
- **强制要求**: 必须使用 `internal/common/common.go` 中的 `common.RunInGoroutine(ctx, func(ctx context.Context) { ... })`，以确保 panic 恢复、日志记录和用户信息传递。

### 3. API 与 Proto 规范
- **接口路径**: 遵循 `/文件夹名/Service小驼峰/接口小驼峰` (例如: `/ai-rag-demo/device/listDevices`)。
- **HTTP 方法**: 纯查询接口使用 `GET`，所有带 Body 参数的接口必须使用 `POST`。
- **枚举命名**: 使用简写格式，移除重复的类型前缀 (例如: `RS_ACCEPTED` 而非 `REPAIR_STATUS_ACCEPTED`)。
- **整数选择**: 优先使用 `int32`。仅在时间戳、文件大小或明确超过 20 亿的数值时使用 `int64`。

### 4. Data 层规范
- **模型命名**: 所有 Data 层模型结构体必须以 `Model` 后缀命名。
- **枚举类型**: DB Model 中对应 proto 枚举的字段，必须直接定义为该枚举类型，严禁定义为 `int32`。
- **GORM查询**: 使用 `GormDB(ctx)` 执行查询 or 操作时，**必须明确调用 `.Model(&XXXModel{})`**，以确保 GORM 能正确识别目标表和钩子（例如：`r.GormDB(ctx).Model(&DeviceModel{}).Create(m)`）。
- **字段备注**: 所有 Model 结构体字段**必须包含单行注释**作为备注（参考 `internal/data/ai-rag-demo/feedback.go`），以便生成数据库文档和代码维护。
- **SQL 位置**: 所有 SQL 查询、条件组装、分页排序等数据库操作**必须写在 `internal/data/` 层**，严禁在 Biz 层或 Service 层直接拼装 SQL 或调用 `Raw`/`Exec` 等。
- **传参规范**: 若某个查询方法所需参数基本都来自接口请求（如 list 搜索接口），**直接将 req 传入 Data 层**，避免在 Biz 层手工拆包再重组。

### 5. 数据库事务规范
- **位置要求**: 涉及多个 Repository 写入或需要保证原子性的复杂业务逻辑，**必须在 `internal/biz/` 层开启事务**，严禁在 Repo 层封装跨表的事务逻辑。
- **开启方式**: 统一使用 `allDb.[模块]DB.InTransaction(ctx, func(ctx context.Context) error { ... })`。
- **连接一致性**: 在事务回调函数内部，调用 Repo 方法时必须传递回调提供的 `ctx`，确保所有操作共享同一个事务连接。

### 6. 获取用户信息规范 
- **强制要求**: 获取当前用户信息必须直接使用 `internal/common/user.go` 中的 `common.UserFromContext(ctx)`，严禁自定义获取逻辑或使用过时的工具函数。 

### 7. gRPC 客户端集成规范
- **集成点**: 所有外部 gRPC 客户端必须集成在 `internal/external/proxy.go` 的 `RPCProxy` 结构体中。
- **初始化**: 在 `NewRPCProxy` 构造函数中统一使用 `RealTimeDiscoveryGrpcClientConn` 进行实例化。
- **引用路径**: 引用生成的 Go 代码时，建议使用模块名作为包别名（如 `ai-rag-demo "ai-rag-demo-manager/api/ai-rag-demo/grpc/ai-rag-demo"`）。
- **完整流程**: 1. 修改 Proto -> 2. `make api` 生成代码 -> 3. 在 `RPCProxy` 中注册字段 -> 4. `make wire` 刷新依赖注入。

## 开发工具链
- **gopls**: 必须利用其索引能力进行跨包引用分析和定义跳转。
- **代码生成**: 修改 Proto 后执行 `make api`；修改 Provider 后执行 `make wire`；全量执行 `make all`。
