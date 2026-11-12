// 截图脚本（一次性原型 #31）：/graph-encoding 三视口
// 必须放在 apps/zhixu-frontend 内运行（node 解析 tailwindcss 等依赖需要本地 node_modules）
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const { spawn } = await import('node:child_process')
const fs = await import('node:fs')

const PORT = 9347
const BASE = 'http://127.0.0.1:5181'
const OUT = '/tmp/zhixu-shots/graph'
fs.mkdirSync(OUT, { recursive: true })

const widths = { d: [1440, 900], t: [768, 1024], m: [390, 844] }
const targets = Object.entries(widths).map(([vw, [w, h]]) => ({
  name: `${vw}_graph_encoding`,
  w,
  h,
  url: `${BASE}/graph-encoding`,
}))

const proc = spawn(
  CHROME,
  [
    '--headless=new',
    '--no-sandbox',
    '--disable-gpu',
    '--disable-crash-reporter',
    '--disable-breakpad',
    '--hide-scrollbars',
    `--remote-debugging-port=${PORT}`,
    '--user-data-dir=/tmp/zhixu-cdp-graph',
    '--window-size=1440,900',
    'about:blank',
  ],
  { stdio: 'ignore' },
)

const sleep = ms => new Promise(r => setTimeout(r, ms))
let target
for (let i = 0; i < 80; i++) {
  try {
    const r = await fetch(`http://127.0.0.1:${PORT}/json/list`)
    const j = await r.json()
    target = j.find(t => t.type === 'page')
    if (target) break
  } catch {}
  await sleep(250)
}
if (!target) {
  console.error('no chrome target')
  proc.kill()
  process.exit(1)
}

const ws = new WebSocket(target.webSocketDebuggerUrl)
await new Promise(r => ws.addEventListener('open', r))
let id = 0
const pending = new Map()
const errors = []
ws.addEventListener('message', e => {
  const m = JSON.parse(e.data)
  if (m.method === 'Runtime.exceptionThrown')
    errors.push(
      'EXC: ' +
        (m.params.exceptionDetails.exception?.description || m.params.exceptionDetails.text),
    )
  if (m.method === 'Runtime.consoleAPICalled' && m.params.type === 'error')
    errors.push('CONSOLE: ' + JSON.stringify(m.params.args?.map(a => a.value ?? a.description)))
  if (m.id && pending.has(m.id)) {
    pending.get(m.id)(m)
    pending.delete(m.id)
  }
})
const send = (method, params = {}) =>
  new Promise(res => {
    const i = ++id
    pending.set(i, res)
    ws.send(JSON.stringify({ id: i, method, params }))
  })

await send('Page.enable')
await send('Runtime.enable')
// mobile 必须为 false：true 会让 390px 视口被算成 980×2121
await send('Emulation.setDeviceMetricsOverride', {
  width: 1440,
  height: 900,
  deviceScaleFactor: 1,
  mobile: false,
})
await send('Page.navigate', { url: `${BASE}/login` })
await sleep(3000)

const token = fs.readFileSync('/tmp/token.txt', 'utf8').trim()
await send('Runtime.evaluate', {
  expression: `localStorage.setItem('user', ${JSON.stringify(
    JSON.stringify({ token, refreshToken: '', profile: null }),
  )})`,
})

for (const t of targets) {
  await send('Emulation.setDeviceMetricsOverride', {
    width: t.w,
    height: t.h,
    deviceScaleFactor: 1,
    mobile: false,
  })
  await send('Page.navigate', { url: t.url })
  await sleep(6000)
  const res = await send('Page.captureScreenshot', { format: 'png', fromSurface: true })
  const file = `${OUT}/${t.name}.png`
  fs.writeFileSync(file, Buffer.from(res.result.data, 'base64'))
  const probe = await send('Runtime.evaluate', {
    expression: `JSON.stringify({w:innerWidth,h:innerHeight,scrollW:document.documentElement.scrollWidth,appLen:document.getElementById('app')?.innerHTML.length ?? 0,svgs:document.querySelectorAll('svg').length,rects:document.querySelectorAll('rect').length,lines:document.querySelectorAll('line').length})`,
    returnByValue: true,
  })
  console.log(t.name, fs.statSync(file).size, probe.result.result.value)
}

console.log('--- errors ---')
console.log(errors.slice(0, 10).join('\n') || '(none)')
ws.close()
proc.kill()
