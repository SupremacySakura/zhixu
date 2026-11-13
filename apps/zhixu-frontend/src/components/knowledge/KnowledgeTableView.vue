<script setup lang="ts">
/**
 * 知识库的紧凑表格视图（#35 用户故事 4 / #40 AC 第 3 条）。
 *
 * 「作为要快速扫读大量条目的用户，我希望除了卡片视图之外还有一个表格视图，
 *   一屏能看到 20 行，以便在已经知道要找什么时不必滚动。」
 *
 * 三条硬约束，决定了这个组件的形状：
 *
 * 1. **一屏 20 行**。行高必须是个能算的常量，不能随内容长。所以标题与关系
 *    摘要都 `truncate` 单行，整行定高 **32px**（`h-8`）。20 行 = 640px + 表头
 *    33px = 673px，1440×900 里装得下：页头带 64 + 工具栏 117 + 表头 33 +
 *    20×32 = 854 < 900（实测 20 行全部落在视口内，表格底边 875px）。
 *    这条是**实测过的**：第一版用 `py-1`（行高 33.5px）时第 20 行底边到 914px，
 *    超出视口 14px、只看得见 19 行，所以把单元格纵向内边距收到 `py-0.5`、
 *    行高钉成 `h-8`。改这个值必须重测「一屏几行」。
 *
 * 2. **无列线**。DESIGN.md §Hairline 纪律：「一律不画表格列与列之间（禁止全网格
 *    ——会读成账本）」。所以只画行间一条 hairline，列与列之间靠留白分组。
 *    行线本身是明文豁免的：「表格行与行之间」是四种允许的交界之一。
 *
 * 3. **不新增任何后端请求**。`items` 与 `relations` 由父页面已经取好，
 *    这里只负责换一种排布。表格视图与卡片视图共用同一份数据、同一份关系摘要、
 *    同一个分页 —— 切换视图不触发重新加载（换视图不是换查询）。
 *
 * 为什么不做排序/筛选：那些已经在工具栏里，表格视图只是「同一批条目的另一种
 * 排布」，在这里再放一套排序入口就是两套状态源。
 */
import { RouterLink } from 'vue-router'
import AppIcon from '../ui/AppIcon.vue'
import KnowledgeBadge from './KnowledgeBadge.vue'
import type { KnowledgeItem } from '../../services/apis/knowledge'
import { knowledgeDate, knowledgeSourceLabel } from '../../utils/knowledge'

/** 交叉引用摘要：由父页面的 `summarize()` 算好，这里只负责印出来。
 *  不 import 父页面的类型 —— `<script setup>` 不能有 ES 导出，而为一个三字段
 *  的形状再开一个 types 文件是过度工程；结构相同即可，TS 会按结构比对。 */
interface RelationSummary {
  total: number
  slips: { type: string; title: string }[]
  failed?: boolean
}

const props = defineProps<{
  items: KnowledgeItem[]
  relations: Record<number, RelationSummary>
}>()

/** 卡片视图左上的分类号（DESIGN.md §Layout 招牌动作「左上分类号 KM-0142」）。
 *  与卡片视图同一套生成规则，两处都从 id 推 —— 表格与卡片看到的是同一个号。 */
const knowledgeSerial = (id: number) => `KM-${String(id).padStart(4, '0')}`

/** 关系列的一句话摘要：`依赖 2 · 矛盾 1`。没有关系就是「—」。
 *  `failed` 必须与「确实没有」分开说 —— 与卡片视图同一条口径，
 *  把读取失败画成破折号就是假阴性。 */
function relationText(id: number) {
  const summary = props.relations[id]
  if (!summary) return '—'
  if (summary.failed) return '读取失败'
  if (!summary.total) return '—'
  const counts = new Map<string, number>()
  for (const slip of summary.slips) counts.set(slip.type, (counts.get(slip.type) ?? 0) + 1)
  const parts = [...counts].map(([type, count]) => `${type} ${count}`)
  const shown = summary.total - summary.slips.length
  // 纸条只带前 3 条；超出部分并入「另 N 条」，避免这一列宽到把标题挤没。
  return shown > 0 ? `${parts.join(' · ')} · 另 ${shown} 条` : parts.join(' · ')
}

/** 分块进度：`12/12`。还没分块时印「—」而不是「0/0」——
 *  后者看起来像「分块了但都是空的」，前者是「还没开始」。 */
function chunkText(item: KnowledgeItem) {
  if (!item.chunks_total) return '—'
  return `${item.chunks_ready}/${item.chunks_total}`
}
</script>

<template>
  <div class="border border-hairline bg-surface shadow-card">
    <!-- 窄屏不做横向滚动（DESIGN.md §Responsive「表格变两列卡列表，不做横向滚动」）：
         这里不是把表格压扁，而是把「来源 / 分块 / 日期 / 关系」四列藏起来，
         只留分类号 + 标题 + 状态，剩下的信息点进详情页看。
         藏列用 `hidden md:table-cell`，不是 `overflow-x-auto`。 -->
    <table class="w-full border-collapse text-left">
      <caption class="sr-only">
        知识条目表格视图，共
        {{
          items.length
        }}
        行
      </caption>
      <thead>
        <tr class="border-b border-rule">
          <th
            scope="col"
            class="w-24 whitespace-nowrap px-3 py-2 font-type text-micro font-normal text-label-tertiary"
          >
            分类号
          </th>
          <th scope="col" class="px-3 py-2 font-type text-micro font-normal text-label-tertiary">
            标题
          </th>
          <th
            scope="col"
            class="hidden w-28 whitespace-nowrap px-3 py-2 font-type text-micro font-normal text-label-tertiary md:table-cell"
          >
            来源
          </th>
          <th
            scope="col"
            class="hidden w-20 whitespace-nowrap px-3 py-2 font-type text-micro font-normal text-label-tertiary lg:table-cell"
          >
            分块
          </th>
          <th
            scope="col"
            class="hidden w-44 whitespace-nowrap px-3 py-2 font-type text-micro font-normal text-label-tertiary md:table-cell"
          >
            日期
          </th>
          <th
            scope="col"
            class="hidden w-40 whitespace-nowrap px-3 py-2 font-type text-micro font-normal text-label-tertiary lg:table-cell"
          >
            交叉引用
          </th>
        </tr>
      </thead>
      <tbody>
        <!-- 行高固定：一屏 20 行是这条用户故事的验收口径，行高一旦跟着内容走
             就永远算不准一屏能放几行。所以所有单元格 `whitespace-nowrap`
             + 标题列 `truncate`，用 `max-w-0` 让 truncate 在表格里真正生效
             （table-layout 下 `max-w-0` 是让单元格不撑开的唯一办法）。 -->
        <tr
          v-for="entry in items"
          :key="entry.id"
          class="h-8 border-b border-hairline last:border-b-0 hover:bg-surface-secondary"
        >
          <td class="px-2.5 py-0.5 align-middle">
            <span class="font-type text-micro text-label-tertiary">{{
              knowledgeSerial(entry.id)
            }}</span>
          </td>
          <!-- 标题即入口：整行唯一可点的地方。表格行不做整行点击 ——
               行里还有收藏与更多操作，整行可点会抢走它们的点击。 -->
          <td class="max-w-0 px-2.5 py-0.5 align-middle">
            <div class="flex min-w-0 items-center gap-2">
              <RouterLink
                :to="`/home/knowledge/${entry.id}`"
                class="min-w-0 truncate text-body text-label underline-offset-2 hover:underline"
                :title="entry.title"
                >{{ entry.title }}</RouterLink
              >
              <KnowledgeBadge :status="entry.status" />
              <span v-if="entry.archived_at" class="flex-none text-micro text-warning">已归档</span>
              <!-- 收藏是状态不是动作：表格里不做收藏开关（那是卡片视图的事），
                   只在已收藏时印一枚星，免得表格与卡片出现两套收藏入口。 -->
              <AppIcon
                v-if="entry.is_favorite"
                icon="material-symbols:star-rounded"
                class="atom-glyph flex-none text-warning"
                aria-hidden="true"
                :style="{ '--atom-glyph-size': '14px' }"
              />
            </div>
          </td>
          <td
            class="hidden truncate whitespace-nowrap px-2.5 py-0.5 align-middle text-note text-label-secondary md:table-cell"
          >
            {{ knowledgeSourceLabel(entry.source_type) }}
          </td>
          <td
            class="hidden whitespace-nowrap px-2.5 py-0.5 align-middle font-type text-note text-label-secondary lg:table-cell"
          >
            {{ chunkText(entry) }}
          </td>
          <td
            class="hidden truncate whitespace-nowrap px-2.5 py-0.5 align-middle font-type text-note text-label-tertiary md:table-cell"
          >
            {{ knowledgeDate(entry.created_at) }}
          </td>
          <td
            class="hidden truncate whitespace-nowrap px-2.5 py-0.5 align-middle text-note text-label-secondary lg:table-cell"
            :class="relations[entry.id]?.failed ? 'text-warning-strong' : ''"
          >
            {{ relationText(entry.id) }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
