// 仅用于设计预览：所有任务和数据均在内存中，不调用真实服务。
const items = [
    {
        id: 'rag', title: 'RAG 学习笔记', name: 'RAG 学习笔记.md', importState: 'ready',
        vectorState: 'partial', chunksTotal: 12, chunksDone: 10, failedChunks: 2,
        vectorError: '供应商请求限流、写入向量库超时', model: 'bge-m3 · 阿里云百炼', version: 'v1',
        joined: '10月23日', updated: '10月23日 18:42',
    },
    {
        id: 'react', title: 'React Server Components', name: 'React Server Components.md', importState: 'ready',
        vectorState: 'ready', chunksTotal: 8, chunksDone: 8, failedChunks: 0,
        model: 'bge-m3 · 阿里云百炼', version: 'v1', joined: '10月22日', updated: '10月22日 09:15',
    },
    {
        id: 'go', title: 'Go 并发模式', name: 'Go 并发模式.markdown', importState: 'ready',
        vectorState: 'running', chunksTotal: 6, chunksDone: 3, failedChunks: 0,
        model: 'bge-m3 · 阿里云百炼', version: 'v1', joined: '10月23日', updated: '10月23日 17:03',
    },
    {
        id: 'raft', title: 'Raft 共识算法笔记', name: 'Raft 共识算法.md', importState: 'ready',
        vectorState: 'pending', chunksTotal: 0, chunksDone: 0, failedChunks: 0,
        model: 'bge-m3 · 阿里云百炼', version: 'v1', joined: '10月23日', updated: '10月23日 16:20',
    },
    {
        id: 'empty', title: '待整理笔记', name: '待整理笔记.md', importState: 'failed',
        importError: '未读取到有效正文。原文件为空，请更新文件后重试。', joined: '10月19日',
    },
]

// 分块演示数据：seq 为写入顺序；ov 表示与上一块重叠 64 字符；section 为所属章节（→ 表示跨节边界）。
const chunks = {
    rag: [
        { seq: 1, section: '理解 RAG：让回答有据可查', chars: 486, state: 'ready', text: '检索增强生成（RAG）将资料检索与大语言模型结合。回答之前，系统先找到相关内容，再把这些内容作为回答依据，而不是只依赖模型在训练时记住的知识。' },
        { seq: 2, section: '理解 RAG：让回答有据可查', chars: 512, state: 'ready', ov: true, text: '再把这些内容作为回答依据。RAG 并不修改模型参数，而是在每次回答前临时检索资料，因此更新资料后无需重新训练，新内容可以立即进入回答流程。' },
        { seq: 3, section: '理解 RAG → 一个清晰的处理流程', chars: 508, state: 'ready', text: '一个清晰的处理流程。整个链路可以拆成四步：保留原始文档并提取可读正文；将正文切分成适合检索的片段；根据问题检索相关片段；最后基于证据回答并标明来源。' },
        { seq: 4, section: '一个清晰的处理流程', chars: 512, state: 'ready', text: '保留原始文档，提取可读正文。切分时优先按标题层级保持语义完整，再按目标长度切分；块与块之间保留一小段重叠，避免句子在边界处被切断。' },
        { seq: 5, section: '一个清晰的处理流程', chars: 497, state: 'ready', ov: true, text: '块与块之间保留一小段重叠，避免句子在边界处被切断。每个片段会被转换成一组向量——一组可以计算相似度的数字，用于在海量片段中快速找到与问题最相关的内容。' },
        { seq: 6, section: '一个清晰的处理流程', chars: 512, state: 'ready', text: '根据问题检索相关片段。检索时会同时考虑向量相似度与关键词匹配，既能理解语义相近的表达，也不会错过术语完全一致的结果。' },
        { seq: 7, section: '一个清晰的处理流程', chars: 503, state: 'failed', error: '供应商请求限流，嵌入调用失败。', text: '基于证据回答，并标明来源。回答中的每个结论都应当能追溯到具体文档与片段，使用者可以打开原文核对上下文，而不是只能相信模型的转述。' },
        { seq: 8, section: '为什么要保留来源', chars: 512, state: 'ready', text: '为什么要保留来源？知识库不是把文件换一个地方存放。它让资料成为可查找、可核对的知识：检索到片段后，使用者可以回到原文确认上下文、作者与时间。' },
        { seq: 9, section: '为什么要保留来源', chars: 489, state: 'ready', ov: true, text: '使用者可以回到原文确认上下文、作者与时间。保留来源还意味着可以评估回答质量：当回答出错时，可以检查是检索到的证据有问题，还是模型在组织语言时出了错。' },
        { seq: 10, section: '为什么要保留来源', chars: 512, state: 'ready', text: '先让知识能够可靠地保存和阅读，再逐步加入智能检索。这个顺序保证了每一步都建立在可信的数据之上，出现问题时也能定位到具体环节。' },
        { seq: 11, section: '小结与实践建议', chars: 356, state: 'failed', error: '写入向量库超时。', text: '小结：RAG 的价值在于“有据可查”。从保存原文、合理分块、可靠嵌入到标明来源，每个环节都影响最终回答的可信度。' },
        { seq: 12, section: '小结与实践建议', chars: 128, state: 'ready', ov: true, text: '每个环节都影响最终回答的可信度。先把链路做稳，再逐步优化检索与排序策略。' },
    ],
    react: [
        { seq: 1, section: 'React Server Components', chars: 512, state: 'ready', text: 'Server Components 允许部分组件在服务端执行。它们可以靠近数据源读取数据，并减少发送给浏览器的 JavaScript 体积，对内容型页面的首屏性能尤其重要。' },
        { seq: 2, section: 'React Server Components', chars: 498, state: 'ready', ov: true, text: '并减少发送给浏览器的 JavaScript 体积。因为服务端组件不参与客户端交互，它们不需要 hydration，也不会增加打包后的 bundle 大小。' },
        { seq: 3, section: 'React Server Components → 服务端与客户端的职责', chars: 512, state: 'ready', text: '服务端与客户端的职责。需要状态、事件处理或浏览器 API 的交互部分，可以使用 Client Components。并不是所有组件都应该放到客户端执行。' },
        { seq: 4, section: '服务端与客户端的职责', chars: 503, state: 'ready', text: '在组件顶部声明 use client 后，该组件及其子组件默认在客户端运行。数据获取、鉴权与重型计算应尽量留在服务端。' },
        { seq: 5, section: '服务端与客户端的职责', chars: 487, state: 'ready', ov: true, text: '数据获取、鉴权与重型计算应尽量留在服务端。这样既能缩短敏感逻辑的暴露面，也能复用服务端的数据库连接与缓存。' },
        { seq: 6, section: '理解边界', chars: 512, state: 'ready', text: '理解边界。在边界之间传递的 props 必须是可序列化的数据；函数、类实例与浏览器对象不能直接跨边界传递。' },
        { seq: 7, section: '理解边界', chars: 445, state: 'ready', ov: true, text: '必须是可序列化的数据。遇到需要跨边界的回调，可以把交互拆成更小的客户端组件，而不是把整个页面标记为客户端组件。' },
        { seq: 8, section: '小结', chars: 210, state: 'ready', text: '小结：先用服务端组件组织内容与数据，再为真正需要交互的部分引入客户端组件。' },
    ],
}

const articles = {
    rag: `<h2>理解 RAG：让回答有据可查</h2><p>检索增强生成（RAG）将资料检索与大语言模型结合。回答之前，系统先找到相关内容，再把这些内容作为回答依据。</p><h3>一个清晰的处理流程</h3><ul><li>保留原始文档，提取可读正文。</li><li>将正文切分成适合检索的片段。</li><li>根据问题检索相关片段。</li><li>基于证据回答，并标明来源。</li></ul><h3>为什么要保留来源？</h3><p>知识库不是把文件换一个地方存放。它让资料成为可查找、可核对的知识；原件则始终用于回溯。</p><blockquote>先让知识能够可靠地保存和阅读，再逐步加入智能检索。</blockquote>`,
    react: `<h2>React Server Components</h2><p>Server Components 允许部分组件在服务端执行。它们可以靠近数据源读取数据，并减少发送给浏览器的 JavaScript。</p><h3>服务端与客户端的职责</h3><p>需要状态、事件处理或浏览器 API 的交互部分，可以使用 Client Components。并不是所有组件都应该放到客户端。</p><h3>理解边界</h3><ul><li>服务端组件负责读取数据与组织内容。</li><li>客户端组件负责交互与浏览器能力。</li><li>在边界之间传递可序列化的数据。</li></ul><blockquote>这份笔记用于理解组件执行位置，不等同于完整的框架迁移指南。</blockquote>`,
    go: `<h2>Go 并发模式</h2><p>使用 goroutine 执行并发工作，通过 channel 协调数据流。并发任务需要明确的退出条件，避免泄漏。</p><h3>任务取消</h3><p>将 context.Context 传入任务，在取消时及时结束处理。</p><h3>有限并发</h3><p>使用固定数量的 worker 控制资源占用，不为每个输入无限创建 goroutine。</p>`,
    raft: `<h2>Raft 共识算法</h2><p>Raft 将一致性问题分解为领导选举、日志复制与安全性三个子问题，通过任期与投票规则保证集群在多数节点存活时继续工作。</p><h3>领导选举</h3><p>每个任期最多一个领导者；候选者必须获得多数票才能当选。</p><h3>日志复制</h3><p>领导者接收客户端请求，先将日志条目复制到多数节点再提交。</p>`,
}

const importLabels = { ready: '已加入', running: '处理中', pending: '排队中', failed: '加入失败' }
const vectorLabels = { pending: '待向量化', running: '向量化中', ready: '已向量化', partial: '部分失败' }
const vectorSteps = ['切分正文并保存分块', '调用嵌入模型', '写入向量库']
const $ = (id) => document.getElementById(id)
const escapeHtml = (value) => String(value).replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char])
let toastTimer

function showToast(message) {
    clearTimeout(toastTimer)
    $('toast-message').textContent = message
    $('toast-wrap').hidden = false
    toastTimer = setTimeout(() => { $('toast-wrap').hidden = true }, 3500)
}

function importBadge(item) {
    const cls = item.importState === 'ready' ? 'ready' : item.importState === 'running' ? 'running' : item.importState === 'failed' ? 'failed' : ''
    return `<span class="status ${cls}">${importLabels[item.importState]}</span>`
}

function vectorBadge(item) {
    if (item.importState === 'failed' || item.vectorState === 'none') return ''
    const cls = item.vectorState === 'ready' ? 'ready' : item.vectorState === 'running' ? 'running' : item.vectorState === 'partial' ? 'partial' : ''
    return `<span class="status ${cls}">${vectorLabels[item.vectorState]}</span>`
}

function describe(item) {
    if (item.importState === 'failed') return escapeHtml(item.importError)
    if (item.vectorState === 'pending') return '正文已入库，等待自动向量化。无需手动操作。'
    if (item.vectorState === 'running') return `正在向量化，已入库 ${item.chunksDone}/${item.chunksTotal} 块。完成后即可按语义检索。`
    if (item.vectorState === 'partial') return `${item.chunksDone}/${item.chunksTotal} 个分块已入库，${item.failedChunks} 个失败（${item.vectorError}）。`
    return `已向量化（${item.chunksTotal} 个分块），可打开分块结构核对切分结果。`
}

function menuIcon(name) {
    return `<svg class="icon" aria-hidden="true"><use href="#i-${name}" /></svg>`
}

function rowActions(item) {
    const first = item.importState === 'ready'
        ? `<a class="knowledge-menu-item" role="menuitem" tabindex="-1" href="#detail/${item.id}">${menuIcon('book')}查看知识</a>`
        : `<button class="knowledge-menu-item" role="menuitem" tabindex="-1" data-action="import-status" data-id="${item.id}">${menuIcon('info')}${item.importState === 'failed' ? '查看失败原因' : '查看进度'}</button>`
    const chunkLink = chunks[item.id]
        ? `<a class="knowledge-menu-item" role="menuitem" tabindex="-1" href="#chunks/${item.id}">${menuIcon('layers')}分块结构</a>` : ''
    return `<button class="more-button" type="button" id="more-${item.id}" data-action="more" data-id="${item.id}" aria-label="更多操作：${escapeHtml(item.title)}" title="更多操作" aria-haspopup="menu" aria-expanded="false" aria-controls="menu-${item.id}">${menuIcon('more')}</button>
    <div class="knowledge-menu" id="menu-${item.id}" role="menu" aria-labelledby="more-${item.id}" hidden>
        ${first}${chunkLink}<div class="knowledge-menu-divider" role="separator"></div>
        <button class="knowledge-menu-item danger" role="menuitem" tabindex="-1" data-action="delete" data-id="${item.id}">${menuIcon('trash')}删除知识</button>
    </div>`
}

let activeMenu = null
let activeTrigger = null
let deleteTarget = null

function closeMenu(restoreFocus = false) {
    if (!activeMenu) return
    activeMenu.hidden = true
    activeTrigger.setAttribute('aria-expanded', 'false')
    if (restoreFocus) activeTrigger.focus()
    activeMenu = null
    activeTrigger = null
}

function positionMenu() {
    if (!activeMenu) return
    const rect = activeTrigger.getBoundingClientRect()
    const height = activeMenu.offsetHeight
    const above = rect.bottom + 8 + height > window.innerHeight - 12
    activeMenu.style.left = `${Math.max(12, Math.min(rect.right - activeMenu.offsetWidth, window.innerWidth - activeMenu.offsetWidth - 12))}px`
    activeMenu.style.top = `${Math.max(12, above ? rect.top - height - 8 : rect.bottom + 8)}px`
    activeMenu.style.transformOrigin = above ? 'bottom right' : 'top right'
}

function openMenu(trigger, keyboard = false, last = false) {
    if (activeTrigger === trigger) { closeMenu(true); return }
    closeMenu()
    activeTrigger = trigger
    activeMenu = $(trigger.getAttribute('aria-controls'))
    activeMenu.hidden = false
    trigger.setAttribute('aria-expanded', 'true')
    positionMenu()
    if (keyboard) {
        const entries = activeMenu.querySelectorAll('[role="menuitem"]')
        entries[last ? entries.length - 1 : 0].focus()
    }
}

function renderLibrary() {
    closeMenu()
    const query = $('knowledge-search').value.trim().toLowerCase()
    const filter = $('knowledge-filter').value
    const visible = items.filter((item) => {
        if (!item.title.toLowerCase().includes(query)) return false
        if (filter === 'all') return true
        if (item.importState === 'failed') return false
        return item.vectorState === filter
    })
    $('knowledge-list').innerHTML = visible.map((item) => `<div class="knowledge-row"><div class="knowledge-copy">
    <div class="badge-line">${importBadge(item)}${vectorBadge(item)}</div>
    <h2>${escapeHtml(item.title)}</h2><p>${describe(item)}</p>
    <div class="knowledge-meta"><span>Markdown 文件</span><span>${escapeHtml(item.name)}</span>${item.chunksTotal ? `<span>分块 ${item.chunksTotal}</span>` : '<span>自动触发</span>'}<span>更新于 ${item.updated}</span></div>
    </div><div class="row-actions">${rowActions(item)}</div></div>`).join('')
    const isEmpty = visible.length === 0
    $('library-empty').hidden = !isEmpty
    if (isEmpty) {
        const pristine = !query && filter === 'all'
        $('library-empty').querySelector('h2').textContent = pristine ? '让第一份资料成为知识' : '没有匹配的知识'
        $('library-empty').querySelector('p').textContent = pristine ? '选择已经上传的 Markdown 文件。正文入库后会自动分块并转向量库。' : '试试其他关键词或切换向量状态。'
    }
}

function statRow(label, value) {
    return `<dt>${label}</dt><dd>${value}</dd>`
}

function renderVecPanel(item) {
    if (item.importState === 'failed') { $('vec-panel').innerHTML = ''; return }
    const rows = [statRow('嵌入模型', item.model), statRow('索引版本', item.version), statRow('触发方式', '正文入库后自动'), statRow('更新时间', item.updated)]
    let badge = vectorBadge(item)
    let message = ''
    let cls = 'vec-msg'
    let action = ''
    if (item.vectorState === 'pending') {
        message = '正文已入库，向量化任务排队中，将自动开始。'
    } else if (item.vectorState === 'running') {
        rows.unshift(statRow('已入库', `${item.chunksDone}/${item.chunksTotal} 块`))
        rows.unshift(statRow('分块', item.chunksTotal))
        message = '正在向量化，按块推进，不使用虚构百分比。'
        action = `<button class="btn btn-secondary btn-sm" data-action="task" data-id="${item.id}">查看进度</button>`
    } else if (item.vectorState === 'partial') {
        rows.unshift(statRow('失败', `${item.failedChunks} 块`))
        rows.unshift(statRow('已入库', `${item.chunksDone}/${item.chunksTotal} 块`))
        rows.unshift(statRow('分块', item.chunksTotal))
        message = `${item.failedChunks} 个分块失败：${item.vectorError}。第一批仅展示失败原因，重试在第二批提供。`
        cls += ' danger'
        action = `<a class="btn btn-tint" href="#chunks/${item.id}">查看分块结构</a>`
    } else {
        rows.unshift(statRow('已入库', `全部 ${item.chunksTotal} 块`))
        rows.unshift(statRow('分块', item.chunksTotal))
        message = '全部分块已写入向量库。'
        action = `<a class="btn btn-tint" href="#chunks/${item.id}">查看分块结构</a>`
    }
    $('vec-panel').innerHTML = `<div class="vec-panel"><div class="vec-head"><h2>向量化</h2>${badge}</div>
    <dl>${rows.join('')}</dl><p class="${cls}">${message}</p>${action}</div>`
}

function renderDetail(item) {
    $('detail-title').textContent = item.title
    $('detail-crumb').textContent = item.title
    $('detail-file').textContent = item.name
    $('detail-joined').textContent = `2026年${item.joined}`
    $('detail-content').innerHTML = articles[item.id] || articles.rag
    $('detail-actions').innerHTML = `<a class="btn btn-secondary" href="./MarkdownKnowledge.html#files">定位原文件 <svg class="icon"><use href="#i-arrow" /></svg></a>${chunks[item.id] ? `<a class="btn btn-tint" href="#chunks/${item.id}">分块结构</a>` : ''}`
    renderVecPanel(item)
}

function chunkCard(chunk) {
    const stateBadge = chunk.state === 'ready' ? '<span class="status ready">已向量化</span>' : `<span class="status failed">失败</span>`
    const body = chunk.ov ? `<mark class="ov">${escapeHtml(chunk.text.slice(0, 16))}</mark>${escapeHtml(chunk.text.slice(16))}` : escapeHtml(chunk.text)
    return `<article class="chunk-card ${chunk.state === 'failed' ? 'is-failed' : ''}" id="chunk-${chunk.seq}">
    <div class="chunk-head"><span class="chunk-seq">#${String(chunk.seq).padStart(2, '0')}</span><span class="chunk-section">${escapeHtml(chunk.section)}</span>
    <span class="chunk-meta"><span class="caption t-tertiary">${chunk.chars} 字符</span>${stateBadge}</span></div>
    ${chunk.ov ? '<div class="ov-strip"><svg class="icon"><use href="#i-info" /></svg>与上一块重叠 64 字符 · 保证边界语义连贯</div>' : ''}
    <p class="chunk-text">${body}</p>
    ${chunk.error ? `<p class="chunk-err"><svg class="icon"><use href="#i-info" /></svg>${escapeHtml(chunk.error)}</p>` : ''}
    </article>`
}

function buildOutline(list) {
    const groups = []
    for (const chunk of list) {
        const head = chunk.section.split(' → ')[0]
        const last = groups[groups.length - 1]
        if (last && last.head === head) last.end = chunk.seq
        else groups.push({ head, start: chunk.seq, end: chunk.seq })
    }
    return groups.map((group) => `<button class="outline-item" data-target="chunk-${group.start}"><strong>${escapeHtml(group.head)}</strong><small>块 ${group.start}–${group.end}</small></button>`).join('')
}

function renderChunks(item) {
    const list = chunks[item.id]
    $('chunks-title').textContent = `${item.title} · 分块结构`
    $('chunks-crumb-link').textContent = item.title
    $('chunks-crumb-link').href = `#detail/${item.id}`
    $('chunks-back').href = `#detail/${item.id}`
    const done = list.filter((chunk) => chunk.state === 'ready').length
    const failed = list.length - done
    const totalChars = list.reduce((sum, chunk) => sum + chunk.chars, 0)
    $('outline-rail').innerHTML = `<span class="rail-title">文档大纲</span>${buildOutline(list)}`
    renderChunkList(item, { done, failed, totalChars })
}

function renderChunkList(item, stats) {
    const list = chunks[item.id]
    const onlyFailed = $('chunk-filter').value === 'failed'
    $('chunks-summary').innerHTML = `<strong>${stats.done}</strong> / ${list.length} 个分块已向量化 · 失败 ${stats.failed} · 共 ${stats.totalChars} 字符`
    const visible = onlyFailed ? list.filter((chunk) => chunk.state === 'failed') : list
    const html = []
    visible.forEach((chunk, index) => {
        html.push(chunkCard(chunk))
        const next = visible[index + 1]
        if (!next) return
        html.push(`<div class="chunk-gap">${next.ov ? '<span class="gap-pill">↕ 重叠 64 字符</span>' : ''}</div>`)
    })
    $('chunk-list').innerHTML = html.join('')
}

function renderPanel(item) {
    $('progress-panel').hidden = false
    $('progress-filename').textContent = item.name
    $('progress-title').textContent = `向量化 · ${item.title}`
    const finished = item.vectorState === 'ready'
    const failed = item.vectorState === 'partial'
    const currentIndex = finished || failed ? vectorSteps.length : item.vectorState === 'pending' ? -1 : item.chunksDone >= item.chunksTotal ? 2 : item.chunksDone > 0 ? 1 : 0
    $('progress-steps').innerHTML = vectorSteps.map((label, index) => `<div class="progress-step ${currentIndex > index ? 'done' : currentIndex === index && !finished && !failed ? 'current' : ''}"><span class="step-dot">${currentIndex > index ? '✓' : index + 1}</span>${label}</div>`).join('')
    $('progress-message').textContent = failed
        ? `${item.failedChunks} 个分块失败：${item.vectorError}。失败块保留原因，重试在第二批提供。`
        : finished ? `${item.chunksTotal} 个分块已全部写入向量库。` : item.vectorState === 'pending' ? '排队中，将自动开始。关闭此面板不会取消任务。' : `已入库 ${item.chunksDone}/${item.chunksTotal} 块。关闭此面板不会取消任务。`
    $('panel-actions').innerHTML = finished || failed ? `<a class="btn btn-tint" href="#chunks/${item.id}">查看分块结构</a>` : item.vectorState === 'running' ? `<button class="btn btn-secondary" data-action="simulate" data-id="${item.id}">推进演示下一步</button>` : ''
}

function navigate() {
    const route = decodeURIComponent(location.hash.slice(1)) || 'library'
    const [page, id] = route.split('/')
    const activePage = ['library', 'detail', 'chunks', 'states'].includes(page) ? page : 'library'
    const item = items.find((entry) => entry.id === id)
    for (const key of ['library', 'detail', 'chunks', 'states']) $(`view-${key}`).hidden = key !== activePage
    document.querySelectorAll('[data-page]').forEach((link) => {
        const selected = link.dataset.page === (activePage === 'detail' || activePage === 'chunks' ? 'library' : activePage)
        link.classList.toggle('active', selected)
        if (selected) link.setAttribute('aria-current', 'page')
        else link.removeAttribute('aria-current')
    })
    $('progress-panel').hidden = true
    if (activePage === 'detail') {
        if (!item || item.importState === 'failed') { location.hash = 'library'; return }
        renderDetail(item)
    }
    if (activePage === 'chunks') {
        if (!item || !chunks[item.id]) { location.hash = 'library'; return }
        renderChunks(item)
    }
    renderLibrary()
    window.scrollTo(0, 0)
}

document.addEventListener('pointerdown', (event) => {
    if (activeMenu && !activeMenu.contains(event.target) && !activeTrigger.contains(event.target)) closeMenu()
})

window.addEventListener('resize', () => closeMenu())
window.addEventListener('scroll', (event) => {
    if (activeMenu && !activeMenu.contains(event.target)) closeMenu()
}, true)

document.addEventListener('keydown', (event) => {
    const trigger = event.target.closest('.more-button')
    if (trigger && ['ArrowDown', 'ArrowUp'].includes(event.key)) {
        event.preventDefault()
        if (activeTrigger !== trigger) openMenu(trigger, true, event.key === 'ArrowUp')
        else activeMenu.querySelector('[role="menuitem"]').focus()
        return
    }
    if (!activeMenu) return
    if (event.key === 'Escape') { event.preventDefault(); closeMenu(true); return }
    if (event.key === 'Tab') { closeMenu(true); return }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const entries = [...activeMenu.querySelectorAll('[role="menuitem"]')]
    const current = entries.indexOf(document.activeElement)
    const index = event.key === 'Home' ? 0 : event.key === 'End' ? entries.length - 1
        : (current + (event.key === 'ArrowDown' ? 1 : -1) + entries.length) % entries.length
    entries[index].focus()
})

$('cancel-delete').addEventListener('click', () => $('delete-dialog').close())
$('confirm-delete').addEventListener('click', () => {
    if (!deleteTarget) return
    const index = items.findIndex((item) => item.id === deleteTarget.id)
    if (index !== -1) items.splice(index, 1)
    $('delete-dialog').close()
    renderLibrary()
    $('knowledge-search').focus()
    showToast('演示：知识已删除，原文件保持不变；刷新设计稿可恢复')
})
$('delete-dialog').addEventListener('close', () => { deleteTarget = null })

document.addEventListener('click', (event) => {
    if (event.target.closest('.knowledge-menu-item')) closeMenu(true)
    const outlineTrigger = event.target.closest('.outline-item')
    if (outlineTrigger) {
        document.getElementById(outlineTrigger.dataset.target)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
        return
    }
    const trigger = event.target.closest('[data-action]')
    if (!trigger) return
    const item = items.find((entry) => entry.id === trigger.dataset.id)
    if (!item) return
    if (trigger.dataset.action === 'more') {
        openMenu(trigger, event.detail === 0)
        return
    }
    if (trigger.dataset.action === 'delete') {
        deleteTarget = item
        $('delete-message').textContent = `将删除「${item.title}」及其处理任务，原文件保持不变。此操作不可撤销。（当前仅为设计演示）`
        $('delete-dialog').showModal()
        $('cancel-delete').focus()
        return
    }
    if (trigger.dataset.action === 'import-status') {
        showToast(item.importError || '正文处理中，请稍后查看。')
        return
    }
    if (trigger.dataset.action === 'simulate') {
        if (item.vectorState !== 'running') return
        item.chunksDone = Math.min(item.chunksDone + 1, item.chunksTotal)
        if (item.chunksDone >= item.chunksTotal) {
            item.vectorState = 'ready'
            showToast('向量化完成，分块已可用于检索')
        }
        renderLibrary()
        renderPanel(item)
        if ($('view-detail').hidden === false && $('detail-title').textContent === item.title) renderDetail(item)
        return
    }
    renderPanel(item)
})

$('close-panel').addEventListener('click', () => { $('progress-panel').hidden = true; document.querySelector('.nav-link.active')?.focus() })
$('knowledge-search').addEventListener('input', renderLibrary)
$('knowledge-filter').addEventListener('change', renderLibrary)
$('chunk-filter').addEventListener('change', () => {
    const route = decodeURIComponent(location.hash.slice(1)) || 'library'
    const [, id] = route.split('/')
    const item = items.find((entry) => entry.id === id)
    if (!item || !chunks[item.id]) return
    const list = chunks[item.id]
    const done = list.filter((chunk) => chunk.state === 'ready').length
    renderChunkList(item, { done, failed: list.length - done, totalChars: list.reduce((sum, chunk) => sum + chunk.chars, 0) })
})
window.addEventListener('hashchange', navigate)
navigate()
