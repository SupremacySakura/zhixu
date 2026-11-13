/**
 * 图标族本地注册入口。
 *
 * 世界：索引卡盒。图标只有一个来源——仓库内的 `src/assets/icons.json`
 * （106 个 material-symbols 字形，非测试全口径：模板 .vue 82 个 +
 * .ts 映射表 23 个 + 1 个死图标改名）。这里把它注册进 @iconify/vue 的
 * 离线存储，之后 `AppIcon` 走 `@iconify/vue/offline`，运行期不再请求网络。
 *
 * 不要改这个文件去"顺手"加图标：新增图标必须先补进 icons.json，
 * 并跑 `node scripts/check-icons.mjs`（模板/映射表引用的图标名与本地
 * 数据的差集，非空即失败）。缺图时 Iconify 只静默输出空节点。
 */
import { addCollection } from '@iconify/vue/offline'
import iconSet from '../assets/icons.json'

addCollection(iconSet)
