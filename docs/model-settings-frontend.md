# 模型设置前端

## 实现

- `/home/settings` 使用现有主布局和 JWT 路由保护，桌面侧栏与移动导航均增加设置入口。
- `pages/ModelSettings.vue` 编排模型配置与服务状态，`useAISettings` 管理真实配置读取、写入、取消与错误状态。
- `services/apis/aiSettings.ts` 对接六个后端接口，契约见 [模型设置后端](./model-settings-backend.md)。DELETE 使用 JSON 正文携带版本。
- `ProviderEditor` 负责预设/自定义连接、模型增删、用途与维度；`SettingsModal` 使用原生 dialog 做焦点限制与背景隔离、motion-v 做入场。这里的 dialog 不是原生下拉菜单。
- 复用 `FilterMenu` 自定义菜单，增加可选 field 样式用于表单，长模型名称换行；菜单支持方向键、Home/End、Escape 和 Tab 关闭。

## 行为

- 供应商目录取自后端，不硬编码原型示例模型，不显示尚未适配的 Anthropic 协议。
- 自定义支持 chat/responses、API Key/无认证，预设地址和协议不可变，其他地址需新建自定义连接。
- 模型清单随供应商整体提交。同名模型由后端保留数据库 ID，空/重复模型和不合法向量维度先在前端校验，最终以服务端规则为准。
- API Key 不回填，不进入 Pinia 或 localStorage。空输入省略 api_key，清除提交 clear_api_key；关闭或成功保存后移除输入，失败保留草稿便于重试。
- 默认模型可分别设置或清空。当前默认 Embedding 模型地址/维度变化、切换或清空默认模型时确认重建，保存不执行重建。显示后端 needs_reindex 标记。
- 写接口携带 expected_version；发生 409 保留表单并提示刷新。编辑器刷新清空输入密钥，保留其他草稿并使用最新版本，需要用户再次保存；服务端继续校验默认引用。
- 删除供应商提供确认，被默认值引用时后端返回冲突并提示先切换。
- 读取与写入互斥，卸载取消进行中请求，避免离开页面后回写。

## 未接入能力

模型发现与实际模型连接测试仍未接入；配置保存成功不代表模型调用通过，Embedding 维度当前仍为用户声明。服务状态已接入 Mastra 与 Qdrant 的只读基础连接检测，刷新、取消及状态边界见 [服务状态](./service-status.md)。

## 验证

前端测试覆盖配置读取、非原生菜单、同维度 Embedding 切换确认、失败保留表单、密钥留空/清除、重复模型校验与 API 参数。类型检查、Lint 和生产构建通过。

浏览器验证未登录访问 `/home/settings` 跳转登录提示并保留 redirect。当前浏览器无登录态，未执行真实账号保存、删除与刷新数据库验证；界面逻辑测试使用模拟 API。Go 代码需重启 pnpm dev 后生效，保存 API Key 前需设置后端加密主密钥。
