---
task_id: auth-profile-logout-20261102
status: completed
created: 2026-11-02
last_reviewed: 2026-11-02
applicable_to: "认证、用户资料、退出登录、侧边栏"
---

# 当前用户资料展示与退出登录

## 背景

项目此前登录后任何地方都看不到「我是谁」，也没有退出登录入口；后端 `AuthService.GetUserByID` 已存在但未暴露 HTTP 接口，`useUserStore` 的 `logout()` 只清本地 token。

## 结果

- 新接口 `GET /api/auth/profile`（JWT 保护）：返回 `vo.ProfileResp{ id, username, email, avatar_url, storage_limit, storage_used, created_at }`，复用既有 `GetUserByID`，不返回密码哈希。未认证 401，用户不存在 500（`CodeServerError`）。
- 前端 user store 新增持久化 `profile`（`UserProfile | null`）与 `setProfile`；`logout()` 同时清空 token、refreshToken、profile。store 不依赖 API 层（避免与 request.ts 拦截器的循环引用），资料由页面拉取后注入。
- `HomePage` 挂载时若已登录则静默 `getProfile()` 刷新资料（失败保留缓存值，401 由请求拦截器统一登出）；`LoginPage` 登录成功后先 `setProfile(null)` 清掉上一个账号的缓存，防止短暂显示旧用户名。
- `SideBar` 底部新增用户区（桌面端与移动端抽屉共用）：头像缺省时用用户名首字符圆形占位（`bg-primary-tint`），右侧退出图标按钮（`aria-label="退出登录"`）emit `logout`；父级 `HomePage` 弹 `ConfirmDialog` 二次确认后 `logout()` 并跳转 `/login`。
- 不做后端 logout 接口：JWT 无状态、无黑名单，登出即丢 token；也不记操作日志。

## 决策与理由

- 退出加二次确认弹窗（复用 `ConfirmDialog`），防止误点；与删除知识同款交互。
- 用户区采用「信息行 + 退出图标按钮」而非下拉菜单：菜单目前只有一项，没有意义；以后加「个人设置」时可再演化为菜单。
- `profile` 持久化：刷新页面后立即有名字显示，后台再静默刷新；代价是切换账号瞬间可能显示旧资料，由 LoginPage 清缓存 + HomePage 强制重拉覆盖。
- SideBar 以 props（userName/userEmail/userAvatar）+ emit(logout) 保持展示组件纯粹，不直接依赖 store。

## 教训

- HomePage 这类含 `<router-view>` 的页面写组件测试时，测试路由不能把 `/` 再指向 HomePage，否则 router-view 递归渲染自身导致 `onMounted` 多次触发；路由表只放跳转目标即可。
- 中间件保护的 handler 测试可直接 `c.Set("user_id", uint(n))` 后裸调 handler（复用同包 `getCurrentUserID`），无需起完整中间件链。

## 候选特性

- 用户区演化为下拉菜单（个人设置、修改密码、头像上传）。
- profile 接口未来可聚合未读通知数等。
- 若未来引入 refresh token 或踢人下线需求，再评估 token 黑名单/版本号机制。

## 验证

- 后端：`go build/vet/test ./...` 全绿；`auth_test.go` 新增 TestProfile_NoUserID/Success（断言字段且不含 password）/ServiceError。
- 前端：typecheck/lint/prettier 全绿；222 测试通过。新增 `stores/__tests__/user.test.ts` profile 两例、`components/bussiness/__tests__/SideBar.test.ts` 三例（展示/隐藏/头像图）、`pages/__tests__/HomePage.test.ts` 五例（挂载拉取、未登录不拉、失败保留缓存、确认退出清态并跳转、取消保留）。
- Swagger 已重新生成（`/auth/profile` 入 docs 三文件）。
