# 本地开发环境事实

运行时代理需要知道、但代码里看不出来的环境事实。写在这里而不是 `docs/brain/`，因为 `docs/brain/` 对 AI 只读。

## 后端限流：60 req/min/IP

`apps/zhixu-backend/internal/middleware/ratelimit.go` 是按 IP 的令牌桶：

| 端点 | 容量 |
|---|---|
| 登录 `/api/auth/*` | **10 req/min** |
| 其余全部 | **60 req/min** |

**为什么必须记住**：知识库首屏 = 1 次 `listKnowledge` + 1 次 `listTags` + 每张卡 1 次 `listKnowledgeRelations`（20 条 = 20 次），**一次首屏就吃掉 22 个配额**。加上 3 秒轮询，很容易触发 429。

触发 429 的表现是：**页面被路由守卫弹回登录页**，看起来像"登录态丢了"，实际是限流。截图脚本连续导航三次就可能撞上。

对策：
- 截图流程先拍便宜页面，知识库放最后
- 每个视口之间等 45 秒让桶回血
- 别用 `page_size` 超 100（会返回 `code 1001 invalid knowledge request`）

## 前端 dev server

- 主实例固定 **5173**；子代理/并行任务用 5180+，避免端口冲突
- 后端默认 9000，前端通过 `DEV_BACKEND_PORT` / `DEV_BACKEND_URL` 改指向
- 测试账号 `uiaudit` / `UiAudit123456`

## 浏览器截图

这台机器**没有 playwright / puppeteer**，只有系统 Chrome。完整配方见 [browser-screenshots.md](./browser-screenshots.md)。

关键三条（踩过的坑）：
1. **必须 `--headless=old`**。`--headless=new` 的 GPU 进程起不来，报 `Inspector.targetCrashed`
2. **先登录一次写 localStorage，再循环改视口**。在循环里反复 navigate 到 `/login` 会被路由守卫弹走，截到登录页
3. **每次导航后校验 `location.pathname`**，确认落点正确再截图

## Go 缓存目录

沙箱下默认的 `~/Library/Caches/go-build` 报 `operation not permitted`。跑 Go 测试要显式指定：

```bash
GOCACHE=/tmp/gocache pnpm test:backend
```

## pnpm / npm

- `npx` 在本机**不可用**（`~/.npm` 不可写）。用 `pnpm exec` 或直接 `./node_modules/.bin/<cmd>`
- 根目录**没有** prettier 配置。`prettier --check .` 在根跑会命中几百个未格式化文件（误导），真正的配置在 `apps/zhixu-frontend/.prettierrc`

## git worktree

并行任务用 `git worktree add /tmp/xxx -b branch`。新 worktree **没有 node_modules**，必须软链：

```bash
ln -s <repo>/node_modules /tmp/xxx/node_modules
ln -s <repo>/apps/zhixu-frontend/node_modules /tmp/xxx/apps/zhixu-frontend/node_modules
```

`git worktree remove --force` 会被安全钩子拦，先手动清理再 `git worktree remove`。`git branch -D` 同样被拦，改用 `-d`。
