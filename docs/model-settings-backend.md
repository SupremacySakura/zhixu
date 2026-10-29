# 用户模型配置后端

## 范围

本轮只实现供应商、模型清单、默认模型的存储与管理，不调用模型、不执行模型发现或连接测试、不接 Qdrant、不下发明文密钥。使用现有 `zhixu` 数据库，启动 AutoMigrate 幂等补建：

| 表 | 内容 |
| --- | --- |
| `model_providers` | 用户、稳定 Provider ID、目录归属、显示名称、地址、协议、认证方式、加密凭据 |
| `provider_models` | 用户、所属供应商数据库 ID、模型 ID、用途、向量维度 |
| `user_ai_settings` | 用户默认模型、检索条数与相似度阈值、配置版本、Embedding 版本、需要重建标记 |

供应商 `(user_id, provider_id)` 唯一，模型 `(provider_id, model_id)` 唯一。均为物理删除。Provider ID 和目录归属保存后不可修改；同名模型在更新时保留数据库 ID。模型清单随供应商 PUT 整体替换，不另设重复的模型 CRUD 接口。

## 接口

以下为后端路径，前端通过 `/api` 代理。全部需要 JWT，用户身份取自服务端上下文，拒绝请求中的 `user_id` 等未知字段。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/settings/ai` | 当前用户供应商、模型和默认设置 |
| GET | `/settings/ai/catalog` | DeepSeek、OpenAI、硅基流动连接预设，不是实时模型清单 |
| POST | `/settings/ai/providers` | 创建供应商和模型，成功 201 |
| PUT | `/settings/ai/providers/:id` | 替换供应商配置和模型清单 |
| DELETE | `/settings/ai/providers/:id` | 删除供应商及模型；JSON 携带 `expected_version` |
| PUT | `/settings/ai/defaults` | 更新默认模型 |

所有写接口返回更新后的完整配置视图，保持 `{code,msg,data}`。未配置用户的 settings 版本与默认 ID 均为 0，providers 为数组。首次 POST 的请求示例：

```json
{
  "provider_id": "my-gateway",
  "name": "我的模型网关",
  "catalog_id": "",
  "base_url": "https://gateway.example.com/v1",
  "protocol": "chat",
  "auth_type": "key",
  "api_key": "<通过请求正文提交的密钥>",
  "models": [
    {"model_id": "chat-model", "kind": "chat", "dimension": 0},
    {"model_id": "embedding-model", "kind": "embedding", "dimension": 1024}
  ],
  "expected_version": 0,
  "confirm_reindex": false
}
```

- `provider_id` 为小写字母开头，长度不超过 64，允许数字、短横线和下划线；每用户最多 32 个供应商。
- 显示名称 1–60 字符；每供应商 1–100 个模型，模型 ID 1–200 字符，不允许大小写不敏感的重复。
- 协议首版为 `chat` 或 `responses`，认证为 `key` 或 `none`；Anthropic 等未适配协议拒绝保存。预设目录须与固定地址和协议一致，其他端点选自定义。
- Embedding 维度为 1–65536 正整数，对话维度为 0。维度目前是用户声明，不代表经过调用验证。
- `api_key` 省略或空字符串表示保留；非空替换；`clear_api_key: true` 清除。无认证自动清除凭据。允许保存无密钥的待配置供应商，但不能将其设为默认模型。
- 返回 `credential_configured` 布尔状态，不返回密钥或密文。
- JSON 最大 256 KiB，错误不回显请求正文、密钥或数据库异常。

默认模型请求：

```json
{
  "chat_model_id": 1,
  "embedding_model_id": 2,
  "rag_top_k": 5,
  "rag_min_score": 0.5,
  "expected_version": 1,
  "confirm_reindex": false
}
```

ID 是 `provider_models.id`，不是供应商 Provider ID 或模型名称。允许 0 表示清除/未配置，支持分步设置；非 0 必须属于当前用户且用途匹配。删除或修改默认模型的用途前须先切换默认值。跨用户默认 ID 无效，不开放他人配置。

`rag_top_k`（检索条数，1–50，默认 5）与 `rag_min_score`（相似度阈值，0.05–1，默认 0.5）随默认模型保存；省略字段则保留已存值，便于旧客户端只更新默认模型。检索时阈值先作为 Qdrant `score_threshold` 下推，Go 组装结果时再兜底过滤一次；全部低于阈值时返回空参考，AI 侧按"知识库中没有找到相关内容"拒答，不再强行凑满条数。存量行由 AutoMigrate 列默认值补齐；读取时遇到越界值（如未迁移的零值）回退默认值。检索参数只来自服务端设置，聊天与内部检索网关请求体中的条数一律忽略。

## 版本与事务

写事务先锁定现有用户行，再读取并校验整个配置，串行处理该用户的供应商修改、删除与默认选择。任一失败全部回滚。每次成功写入 `config_version` 加一；客户端提交读取到的 `expected_version`，不匹配返回 409。

默认 Embedding 的供应商、基础地址、模型或维度变化，已有默认值时需 `confirm_reindex: true`；确认后 `embedding_version` 加一，`needs_reindex` 置 true，不自动生成向量。首次选择版本加一但不标记旧索引重建。清空已有默认 Embedding 也需要确认。密钥替换、显示名称和无关模型修改不触发重建。标记由后续索引发布机制清除，本轮无清除接口。

模型快照与持久化任务、向量删除一致性在下一轮接入，当前版本字段仅作为准备。

## 密钥与地址安全

设置 `MODEL_CONFIG_ENCRYPTION_KEY` 为 32 字节随机密钥的标准 Base64 编码：

```bash
openssl rand -base64 32
```

将生成值通过部署配置写入环境变量或本机 `.env.development`，不要提交真实密钥。不要复用 JWT 密钥。AES-256-GCM 使用随机 nonce，认证附加数据绑定用户 ID 和 Provider ID，防止密文跨用户搬移。主密钥缺失时允许基础业务与无密钥配置，提交新密钥返回 503；非空格式错误启动失败。主密钥须与数据库一并备份但分开安全保存，直接更换会导致旧凭据不可解密，本轮不实现轮换工具。

地址保存仅做结构校验，不发网络请求。允许 HTTP(S)，拒绝 URL 内嵌凭据、查询参数、片段、具体调用路径、多播/未指定/链路本地 IP；本机地址可保存以支持原生本地模型。**保存通过不代表允许请求该地址**：后续模型发现/测试/调用必须加入 DNS 解析、重定向、私网与云元数据防护，并通过部署侧精确允许名单开放本地端点。本轮没有可触发 SSRF 的调用入口。

## 验证与差异

`go test ./...`、`go vet ./...`、`go build ./...`，新增测试覆盖加密、凭据清除、用户隔离、默认用途、重建确认、并发版本冲突、事务失败回滚和 HTTP 输入/输出。测试使用 sqlmock 与内存仓储，不连接开发数据库；真实 MySQL 迁移和 JWT HTTP 联调尚未执行。

本实现将原计划单一 `model_configs` 细化为三张表。设计稿含 Anthropic 示例和模拟连接测试，当前后端未提供这些能力，不应按原型声称已支持。Brain 中旧项目名称与测试现状仍过时，按只读协议未修改。

Go 无热更新；重启 `pnpm dev` 后才加载接口和迁移。AI 服务与前端代码本轮未修改。
