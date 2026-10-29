<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import FilterMenu from '../components/ui/FilterMenu.vue'
import SettingsModal from '../components/ui/SettingsModal.vue'
import ProviderEditor from '../components/ui/ProviderEditor.vue'
import ServiceStatusPanel from '../components/ui/ServiceStatusPanel.vue'
import { useServiceStatus } from '../composables/useServiceStatus'
import ToastNotice from '../components/file-management/ToastNotice.vue'
import { useAISettings } from '../composables/useAISettings'
import { useToast } from '../composables/useToast'
import type {
  AIProvider,
  ProviderPreset,
  ProviderPayload,
  ModelKind,
} from '../services/apis/aiSettings'
const {
  configuration,
  catalog,
  loading,
  busy,
  error,
  mutationError,
  conflict,
  load,
  clearError,
  saveProvider,
  removeProvider,
  saveDefaults,
} = useAISettings()
const { displayToast, showToast, toastMessage, toastType } = useToast()
const tab = ref<'models' | 'services'>('models')
const {
  report: serviceReport,
  loading: serviceLoading,
  error: serviceError,
  stale: serviceStale,
  refresh: refreshServices,
} = useServiceStatus(computed(() => tab.value === 'services'))
const catalogOpen = ref(false)
const editorOpen = ref(false)
const editorRef = ref<InstanceType<typeof ProviderEditor> | null>(null)
const editingProvider = ref<AIProvider | null>(null)
const editingPreset = ref<ProviderPreset | null>(null)
const deleteTarget = ref<AIProvider | null>(null)
const chat = ref('0')
const embedding = ref('0')
const ragTopK = ref(5)
const ragMinScore = ref(0.5)
const confirmDefaults = ref(false)
watch(configuration, value => {
  if (!value) return
  chat.value = String(value.settings.chat_model_id)
  embedding.value = String(value.settings.embedding_model_id)
  // Settings rows predating the retrieval columns report zero; mirror the backend defaults.
  ragTopK.value = value.settings.rag_top_k || 5
  ragMinScore.value = value.settings.rag_min_score || 0.5
})
function options(kind: ModelKind) {
  return [
    { key: '0', label: '暂不配置' },
    ...(configuration.value?.providers.flatMap(p =>
      p.models
        .filter(m => m.kind === kind)
        .map(m => ({
          key: String(m.id),
          label: `${p.name} / ${m.model_id}${p.auth_type === 'key' && !p.credential_configured ? '（待配置密钥）' : ''}`,
        })),
    ) || []),
  ]
}
const embeddingModel = computed(() =>
  configuration.value?.providers.flatMap(p => p.models).find(m => m.id === Number(embedding.value)),
)
const dirty = computed(
  () =>
    configuration.value &&
    (Number(chat.value) !== configuration.value.settings.chat_model_id ||
      Number(embedding.value) !== configuration.value.settings.embedding_model_id ||
      Math.round(ragTopK.value) !== (configuration.value.settings.rag_top_k || 5) ||
      Math.round(ragMinScore.value * 100) / 100 !==
        (configuration.value.settings.rag_min_score || 0.5)),
)
function openEditor(provider: AIProvider | null = null, preset: ProviderPreset | null = null) {
  clearError()
  editingProvider.value = provider
  editingPreset.value = preset
  catalogOpen.value = false
  editorOpen.value = true
}
function closeEditor() {
  if (!busy.value) {
    editorOpen.value = false
    clearError()
  }
}
async function submitProvider(payload: ProviderPayload) {
  if (await saveProvider(editingProvider.value?.id ?? null, payload)) {
    editorOpen.value = false
    displayToast('供应商配置已保存', 'success')
  }
}
async function reloadEditor() {
  if (await load()) editorRef.value?.acceptVersion()
}
function requestDefaults() {
  clearError()
  if (
    configuration.value?.settings.embedding_model_id &&
    Number(embedding.value) !== configuration.value.settings.embedding_model_id
  )
    confirmDefaults.value = true
  else void submitDefaults(false)
}
async function submitDefaults(confirmed: boolean) {
  if (!configuration.value) return
  const topK = Math.round(ragTopK.value)
  const score = Math.round(ragMinScore.value * 100) / 100
  if (!Number.isFinite(topK) || topK < 1 || topK > 50) {
    displayToast('参考条数需为 1–50 的整数', 'error')
    return
  }
  if (!Number.isFinite(score) || score < 0.05 || score > 1) {
    displayToast('相似度阈值需在 0.05–1 之间', 'error')
    return
  }
  if (
    await saveDefaults({
      chat_model_id: Number(chat.value),
      embedding_model_id: Number(embedding.value),
      rag_top_k: topK,
      rag_min_score: score,
      expected_version: configuration.value.settings.config_version,
      confirm_reindex: confirmed,
    })
  ) {
    confirmDefaults.value = false
    displayToast('默认模型已保存', 'success')
  }
}
function chooseDelete(provider: AIProvider) {
  clearError()
  deleteTarget.value = provider
}
async function confirmDelete() {
  if (deleteTarget.value && (await removeProvider(deleteTarget.value.id))) {
    deleteTarget.value = null
    displayToast('供应商及模型已删除', 'success')
  }
}
function closeConfirmation() {
  if (!busy.value) {
    deleteTarget.value = null
    confirmDefaults.value = false
    clearError()
  }
}
onMounted(() => {
  void load()
})
function reloadConfirmation() {
  closeConfirmation()
  void load()
}
</script>
<template>
  <main class="overflow-y-auto p-4 sm:p-6 lg:p-8">
    <div class="mx-auto max-w-6xl">
      <p class="mb-4 text-subhead text-label-secondary">工作空间 / 设置</p>
      <header class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 class="text-title-1">让 AI 按你的方式工作</h1>
          <p class="mt-2 text-subhead text-label-secondary">
            连接你信任的模型服务，为知识整理与语义检索做好准备。
          </p>
        </div>
        <span class="rounded-full bg-surface-tertiary px-3 py-1 text-caption text-label-secondary"
          >仅本人可见</span
        >
      </header>
      <nav class="mb-7 mt-7 flex gap-7 border-b border-hairline" aria-label="设置分类">
        <button
          v-for="entry in [
            { key: 'models', label: '模型配置' },
            { key: 'services', label: '服务状态' },
          ]"
          :key="entry.key"
          type="button"
          class="border-b-2 pb-3 text-subhead font-semibold"
          :class="
            tab === entry.key
              ? 'border-primary text-primary-pressed'
              : 'border-transparent text-label-secondary'
          "
          :aria-pressed="tab === entry.key"
          @click="tab = entry.key as typeof tab"
        >
          {{ entry.label }}
        </button>
      </nav>
      <ServiceStatusPanel
        v-if="tab === 'services'"
        :report="serviceReport"
        :loading="serviceLoading"
        :error="serviceError"
        :stale="serviceStale"
        @refresh="refreshServices"
      />
      <section v-else>
        <p v-if="loading" role="status" class="py-8 text-subhead text-label-secondary">
          正在加载模型设置…
        </p>
        <div
          v-if="error"
          role="alert"
          class="mb-5 rounded-md bg-warning-tint p-4 text-subhead text-warning-strong"
        >
          {{ error }}
          <button type="button" class="knowledge-text-action" @click="load">重新加载</button>
        </div>
        <template v-if="configuration">
          <fieldset :disabled="busy || loading" class="min-w-0">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 class="text-title-3">默认模型</h2>
                <p class="mt-2 text-caption text-label-secondary">
                  先连接供应商，再为两类任务分别选择模型。
                </p>
              </div>
              <span class="text-caption text-label-secondary">{{
                dirty ? '有未保存的选择' : '配置版本 ' + configuration.settings.config_version
              }}</span>
            </div>
            <div class="rounded-lg bg-surface shadow-card">
              <div class="grid md:grid-cols-2">
                <div class="min-w-0 border-b border-hairline p-6 md:border-b-0 md:border-r">
                  <h3 class="text-headline">对话与整理</h3>
                  <p class="mb-4 mt-2 text-caption text-label-secondary">
                    摘要、知识整理和引用问答
                  </p>
                  <FilterMenu
                    field
                    v-model="chat"
                    :options="options('chat')"
                    label="选择默认对话模型"
                  />
                </div>
                <div class="min-w-0 p-6">
                  <h3 class="text-headline">向量化与检索</h3>
                  <p class="mb-4 mt-2 text-caption text-label-secondary">
                    知识分块与查询使用同一 Embedding 模型
                  </p>
                  <FilterMenu
                    field
                    v-model="embedding"
                    :options="options('embedding')"
                    label="选择默认 Embedding 模型"
                  />
                  <p class="mt-3 text-caption text-label-secondary">
                    {{
                      embeddingModel
                        ? `${embeddingModel.dimension} 维 · 尚未经实际调用验证`
                        : '尚未选择，暂不可执行向量化。'
                    }}
                  </p>
                  <div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <div>
                      <label for="rag-top-k" class="mb-2 block text-caption font-semibold"
                        >参考条数 TopK</label
                      ><input
                        id="rag-top-k"
                        v-model.number="ragTopK"
                        class="knowledge-input w-full"
                        type="number"
                        min="1"
                        max="50"
                        step="1"
                      />
                    </div>
                    <div>
                      <label for="rag-min-score" class="mb-2 block text-caption font-semibold"
                        >相似度阈值</label
                      ><input
                        id="rag-min-score"
                        v-model.number="ragMinScore"
                        class="knowledge-input w-full"
                        type="number"
                        min="0.05"
                        max="1"
                        step="0.05"
                      />
                    </div>
                  </div>
                  <p class="mt-3 text-caption text-label-secondary">
                    相似度低于阈值的片段不会作为参考；与知识库无关的问题将视为未找到相关内容。条数越大上下文越长（每片约
                    1024 字符），按需调高。
                  </p>
                </div>
              </div>
              <footer
                class="flex flex-wrap items-center justify-between gap-3 border-t border-hairline px-6 py-4"
              >
                <p class="text-caption text-label-secondary">
                  更换 Embedding 模型需要重建向量，不会自动执行。
                </p>
                <button
                  type="button"
                  class="knowledge-button-secondary"
                  :disabled="!dirty"
                  @click="requestDefaults"
                >
                  保存默认模型
                </button>
              </footer>
            </div>
            <p
              v-if="configuration.settings.needs_reindex"
              role="status"
              class="mt-4 rounded-md bg-warning-tint p-4 text-subhead text-warning-strong"
            >
              Embedding 配置已变更，关联向量需要重建。向量化与重建功能尚未接入，知识正文不受影响。
            </p>
            <div class="mb-4 mt-8 flex flex-wrap items-center justify-between gap-4">
              <div>
                <h2 class="text-title-3">模型供应商</h2>
                <p class="mt-2 text-caption text-label-secondary">
                  一个连接管理多个模型，不同任务可以使用不同供应商。
                </p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button
                  type="button"
                  class="knowledge-button-secondary"
                  @click="catalogOpen = true"
                >
                  ＋ 添加供应商</button
                ><button type="button" class="knowledge-button-primary" @click="openEditor()">
                  添加自定义 API
                </button>
              </div>
            </div>
            <div
              v-if="!configuration.providers.length"
              class="rounded-lg bg-surface p-10 text-center shadow-card"
            >
              <h3 class="text-title-3">连接你的第一个模型服务</h3>
              <p class="mt-3 text-subhead text-label-secondary">
                选择第三方供应商，或使用自己的 API 网关与本地模型。
              </p>
            </div>
            <div class="space-y-3">
              <article
                v-for="provider in configuration.providers"
                :key="provider.id"
                class="rounded-lg bg-surface p-5 shadow-card sm:p-6"
              >
                <div class="flex flex-wrap items-center gap-4">
                  <span
                    class="flex h-11 w-11 shrink-0 items-center justify-center rounded-md bg-surface-secondary text-headline font-bold"
                    >{{ provider.catalog_id ? provider.name.charAt(0) : '&lt;/&gt;' }}</span
                  >
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <h3 class="text-headline">{{ provider.name }}</h3>
                      <span
                        class="rounded-full px-3 py-1 text-caption"
                        :class="
                          provider.auth_type === 'none' || provider.credential_configured
                            ? 'bg-primary-tint text-primary-pressed'
                            : 'bg-warning-tint text-warning-strong'
                        "
                        >{{
                          provider.auth_type === 'none'
                            ? '无认证'
                            : provider.credential_configured
                              ? '密钥已配置'
                              : '待配置密钥'
                        }}</span
                      >
                    </div>
                    <p class="mt-2 break-all text-caption text-label-secondary">
                      {{ provider.provider_id }} ·
                      {{ provider.catalog_id ? '预设供应商' : '自定义 API' }} ·
                      {{ provider.base_url }}
                    </p>
                  </div>
                  <div class="flex gap-3">
                    <button
                      type="button"
                      class="knowledge-button-secondary"
                      :aria-label="`配置 ${provider.name}`"
                      @click="openEditor(provider)"
                    >
                      配置</button
                    ><button
                      type="button"
                      class="knowledge-text-action text-danger"
                      :aria-label="`删除 ${provider.name}`"
                      @click="chooseDelete(provider)"
                    >
                      删除
                    </button>
                  </div>
                </div>
                <div class="mt-4 flex flex-wrap gap-2 border-t border-hairline pt-4">
                  <span
                    v-for="model in provider.models"
                    :key="model.id"
                    class="break-all rounded-sm bg-surface-secondary px-3 py-1.5 text-caption"
                    ><span class="mr-2 text-label-secondary">{{
                      model.kind === 'chat' ? '对话' : '向量'
                    }}</span
                    >{{ model.model_id
                    }}<span v-if="model.kind === 'embedding'" class="ml-2 text-label-secondary"
                      >{{ model.dimension }} 维</span
                    ></span
                  >
                </div>
              </article>
            </div>
          </fieldset>
          <div
            v-if="mutationError && !editorOpen && !deleteTarget && !confirmDefaults"
            role="alert"
            class="mt-5 text-subhead text-danger"
          >
            {{ mutationError
            }}<button
              v-if="conflict"
              type="button"
              class="knowledge-text-action ml-3"
              @click="load"
            >
              重新加载最新配置
            </button>
          </div>
          <p class="mt-6 flex gap-3 text-caption leading-6 text-label-secondary">
            <Icon
              icon="material-symbols:shield-lock-outline"
              class="mt-1 shrink-0 text-primary"
            /><span
              ><strong class="font-medium text-label">密钥只写，不回显。</strong
              >凭据由服务端加密保存，不进入浏览器本地存储。配置保存不代表模型连接已验证。</span
            >
          </p>
        </template>
      </section>
    </div>
    <SettingsModal v-if="catalogOpen" title="选择模型供应商" @close="catalogOpen = false">
      <p class="mb-5 text-subhead text-label-secondary">
        预设填入基础地址与协议，模型 ID 需自行添加。
      </p>
      <div class="space-y-3">
        <button
          v-for="preset in catalog"
          :key="preset.provider_id"
          type="button"
          class="flex w-full items-center justify-between rounded-md border border-hairline p-4 text-left hover:bg-surface-secondary disabled:opacity-50"
          :disabled="configuration?.providers.some(p => p.provider_id === preset.provider_id)"
          @click="openEditor(null, preset)"
        >
          <div>
            <strong>{{ preset.name }}</strong>
            <p class="mt-2 break-all text-caption text-label-secondary">{{ preset.base_url }}</p>
          </div>
          <span>{{
            configuration?.providers.some(p => p.provider_id === preset.provider_id)
              ? '已添加'
              : '＋'
          }}</span>
        </button>
      </div>
      <button type="button" class="knowledge-button-primary mt-5" @click="openEditor()">
        添加自定义 API
      </button>
    </SettingsModal>
    <ProviderEditor
      v-if="editorOpen && configuration"
      ref="editorRef"
      :provider="editingProvider"
      :preset="editingPreset"
      :configuration="configuration"
      :busy="busy || loading"
      :error="mutationError || error"
      :conflict="conflict"
      @close="closeEditor"
      @submit="submitProvider"
      @reload="reloadEditor"
    />
    <SettingsModal
      v-if="deleteTarget || confirmDefaults"
      :title="deleteTarget ? '删除供应商' : '切换向量化模型？'"
      :busy="busy"
      @close="closeConfirmation"
    >
      <p class="mb-5 text-subhead leading-7 text-label-secondary">
        {{
          deleteTarget
            ? `将删除「${deleteTarget.name}」的连接和模型清单，不删除知识正文。正在被默认模型使用的供应商须先切换默认值。`
            : '新旧模型的向量不能混用，即使维度相同。保存后关联索引需要重建，本次不会自动重建，知识正文保持不变。'
        }}
      </p>
      <p v-if="mutationError" role="alert" class="mb-4 text-subhead text-danger">
        {{ mutationError }}
      </p>
      <button
        v-if="conflict"
        type="button"
        class="knowledge-text-action mb-4"
        :disabled="busy"
        @click="reloadConfirmation"
      >
        关闭并加载最新配置
      </button>
      <div class="flex justify-end gap-3">
        <button
          type="button"
          class="knowledge-button-secondary"
          :disabled="busy"
          @click="closeConfirmation"
        >
          取消</button
        ><button
          type="button"
          class="knowledge-button-primary"
          :disabled="busy"
          @click="deleteTarget ? confirmDelete() : submitDefaults(true)"
        >
          {{ busy ? '处理中…' : deleteTarget ? '删除供应商' : '确认切换' }}
        </button>
      </div>
    </SettingsModal>
    <ToastNotice :visible="showToast" :message="toastMessage" :type="toastType" />
  </main>
</template>
