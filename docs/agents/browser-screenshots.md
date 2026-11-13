# 浏览器截图配方

给本仓库做视觉验证时的标准做法。这台机器**没有 playwright / puppeteer**，只有系统 Chrome + Node 内置的 CDP 客户端。

## 为什么需要这份文档

视觉改动（CSS 令牌、布局、断点、图标、字体）用测试验不出来。实测过：图标字形丢掉 `fill="currentColor"`、引用了一个不存在的图标名、容器宽度变了 148px——这些全都**typecheck 绿、测试全绿、构建成功**，只有截图能发现。

## 一、最简单的一次性截图

不需要登录态的页面（登录页、注册页、取件页）：

```bash
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --headless=old --no-sandbox --disable-gpu \
  --disable-crash-reporter --disable-breakpad --hide-scrollbars \
  --user-data-dir=/tmp/cr-$$ \
  --screenshot=/tmp/shot.png \
  --window-size=1440,900 \
  --virtual-time-budget=6000 \
  http://127.0.0.1:5173/login
```

**必须 `--headless=old`**。`--headless=new` 在这台机器上 GPU 进程起不来，报 `Inspector.targetCrashed`。

## 二、需要登录态：写 localStorage

`App.vue` 从 `localStorage.user` 读 token。先拿 token：

```bash
curl -s -X POST http://127.0.0.1:5173/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"uiaudit","password":"UiAudit123456"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])'
```

用 CDP 打开 `/login`，等页面起来，然后注入：

```js
localStorage.setItem('user', JSON.stringify({ token, refreshToken: '', profile: null }))
```

再导航到目标页。

## 三、多视口批量截图（最容易踩坑的部分）

**错误做法**（会截到登录页）：在循环里对每个视口都重新导航到 `/login` 并重新登录。路由守卫会在某些时序下把你弹回登录页，而且每次导航都吃限流配额。

**正确做法**：

1. 先在**一个**视口登录一次，写入 localStorage
2. 然后循环只调 `Emulation.setDeviceMetricsOverride` 改视口尺寸
3. **每次导航后校验落点**：

```js
const path = await Runtime.evaluate({ expression: 'location.pathname' })
if (path.result.value !== expected) throw new Error(`落点错误: ${path.result.value}`)
```

4. `Emulation.setDeviceMetricsOverride` 的 `mobile` 参数**必须 `false`**。传 `true` 会让 390px 的视口算成 980×2121，截图完全不对。

## 四、三档标准视口

| 档位 | 尺寸 | 对应断点 |
|---|---|---|
| 移动 | 375×667 | base（<768） |
| 平板 | 768×1024 | md（768–1023） |
| 桌面 | 1440×900 | lg（≥1024） |

另外单点验 **1024×768**：这是 `lg` 的边界，用来确认"侧栏恰出现、抽屉恰消失"。

## 五、与限流共存

后端 **60 req/min/IP**（详见 [environment.md](./environment.md)）。截图流程本身会消耗配额：

- 先拍便宜页面，知识库这类首屏 22 个请求的放最后
- 视口之间隔 45 秒让令牌桶回血
- 出现"被弹回登录页"时，先怀疑 429，不要怀疑登录态

## 六、怎么判断"截图变了是不是回归"

单看截图不一样没有意义，要做**连通域分析**：找出变化的像素块，逐块判断。

实测案例：`/home/dashboard` 两张截图 sha256 不同，连通域分析后发现两处差异——一处是图标之前根本没渲染（修好后才出现，是修复不是回归），一处是操作日志的相对时间戳从"2 小时前"变成"7 小时前"（页面数据，与代码无关）。

**带随机/动画的页面不能靠 sha256 判等**。ECharts 开了 `layoutAnimation` 时，同一份产物连拍两次 sha256 就不同。

## 七、断网验证

验证资源真的本地化了（字体、图标），用 CDP `Network.setBlockedURLs`：

```js
await Network.enable()
await Network.setBlockedURLs({
  urls: ['*fonts.googleapis.com*', '*fonts.gstatic.com*', '*api.iconify.design*', '*unpkg.com*', '*cdn*']
})
```

然后导航并截图。**判定标准不是"没报错"**，而是：
- `document.querySelectorAll('svg').length` 与实际图标数一致
- 空 SVG 数量为 0
- 截图与联网态**逐字节相同**（`sha256` 相等）

缺图标时 Iconify 只静默输出空节点，不报错、不告警、测试全绿。

## 八、手工注入错误态

验证错误提示、空态、失败态时，用 CDP `Fetch` 域把某个接口改写成 500：

```js
await Fetch.enable({ patterns: [{ urlPattern: '*api/auth/profile*' }] })
// Fetch.requestPaused 时回 Fetch.fulfillRequest，responseCode: 500
```

拦在浏览器层，与真实网络错误走同一条路径。**不需要起代理、不需要第二个 dev server。**
