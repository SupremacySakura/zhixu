<script setup lang="ts">
import { ref, computed } from 'vue'
import AppBadge from './AppBadge.vue'
import AppButton from './AppButton.vue'
import FilterMenu from './FilterMenu.vue'
import SettingsModal from './SettingsModal.vue'
import type {
  AIProvider,
  ProviderPreset,
  ProviderPayload,
  AIConfiguration,
  ModelKind,
  ModelProtocol,
} from '../../services/apis/aiSettings'
const props = defineProps<{
  provider: AIProvider | null
  preset: ProviderPreset | null
  configuration: AIConfiguration
  busy: boolean
  error: string
  conflict: boolean
}>()
const emit = defineEmits<{ close: []; submit: [payload: ProviderPayload]; reload: [] }>()
const source = props.provider || props.preset
const providerID = ref(source?.provider_id || '')
const name = ref(source?.name || '')
const baseURL = ref(source?.base_url || '')
const protocol = ref<ModelProtocol>(source?.protocol || 'chat')
const auth = ref<'key' | 'none'>(source?.auth_type || 'key')
const apiKey = ref('')
const clearKey = ref(false)
const models = ref(
  (props.provider?.models || []).map(m => ({
    model_id: m.model_id,
    kind: m.kind,
    dimension: m.dimension,
    key: crypto.randomUUID(),
  })),
)
if (!models.value.length)
  models.value.push({ model_id: '', kind: 'chat', dimension: 0, key: crypto.randomUUID() })
const catalogID = props.provider?.catalog_id || props.preset?.provider_id || ''
const localError = ref('')
const reindexConfirmed = ref(false)
const version = ref(props.configuration.settings.config_version)
const currentProvider = computed(() =>
  props.configuration.providers.find(p => p.id === props.provider?.id),
)
const oldEmbedding = computed(() =>
  currentProvider.value?.models.find(m => m.id === props.configuration.settings.embedding_model_id),
)
const needsConfirmation = computed(() => {
  if (!oldEmbedding.value) return false
  const next = models.value.find(
    m => m.model_id.trim() === oldEmbedding.value!.model_id && m.kind === 'embedding',
  )
  return (
    !!next &&
    (baseURL.value.trim().replace(/\/+$/, '') !== currentProvider.value?.base_url ||
      next.dimension !== oldEmbedding.value.dimension)
  )
})
function clearCredential() {
  apiKey.value = ''
  clearKey.value = true
}
function changeAuth(value: string) {
  auth.value = value as 'key' | 'none'
  if (auth.value === 'none') clearCredential()
}
function changeKind(index: number, value: string) {
  const entry = models.value[index]!
  entry.kind = value as ModelKind
  entry.dimension = 0
}
function addModel() {
  models.value.push({ model_id: '', kind: 'chat', dimension: 0, key: crypto.randomUUID() })
}
function submit() {
  localError.value = ''
  const normalized = models.value.map(({ model_id, kind, dimension }) => ({
    model_id: model_id.trim(),
    kind,
    dimension: kind === 'embedding' ? dimension : 0,
  }))
  if (
    !normalized.length ||
    normalized.some(m => !m.model_id) ||
    new Set(normalized.map(m => m.model_id.toLowerCase())).size !== normalized.length
  ) {
    localError.value = '请添加至少一个模型，模型 ID 不能为空或重复。'
    return
  }
  if (
    normalized.some(
      m =>
        m.kind === 'embedding' &&
        (!Number.isInteger(m.dimension) || m.dimension < 1 || m.dimension > 65536),
    )
  ) {
    localError.value = 'Embedding 维度须为 1–65536 的整数。'
    return
  }
  if (needsConfirmation.value && !reindexConfirmed.value) {
    localError.value = '请确认当前 Embedding 配置变更需要重建向量。'
    return
  }
  emit('submit', {
    provider_id: providerID.value.trim(),
    name: name.value.trim(),
    catalog_id: catalogID,
    base_url: baseURL.value.trim(),
    protocol: protocol.value,
    auth_type: auth.value,
    models: normalized,
    expected_version: version.value,
    confirm_reindex: needsConfirmation.value && reindexConfirmed.value,
    ...(apiKey.value && auth.value === 'key' ? { api_key: apiKey.value } : {}),
    ...(clearKey.value && !apiKey.value ? { clear_api_key: true } : {}),
  })
}
function close() {
  apiKey.value = ''
  emit('close')
}
// Reload keeps the local draft; the next save explicitly uses the refreshed version.
async function reload() {
  apiKey.value = ''
  emit('reload')
}
function acceptVersion() {
  version.value = props.configuration.settings.config_version
}
defineExpose({ acceptVersion })
</script>
<template>
  <SettingsModal
    :title="provider ? `配置 ${provider.name}` : preset ? `连接 ${preset.name}` : '添加自定义 API'"
    :busy="busy"
    @close="close"
  >
    <p class="mb-5 text-body leading-6 text-label-secondary">
      填写连接信息，再手动添加可用模型。保存配置不会调用模型。
    </p>
    <form @submit.prevent="submit">
      <!-- 字段间距 24px（DESIGN §Layout ④表单）。
           这些字段全部保留裸 <input>：AppInput 会把 id 同时落到外层 div，
           ProviderEditor 的三条 get('#provider-key') 与
           get('#provider-id') 就会取到 DIV，setValue 直接抛。 -->
      <fieldset :disabled="busy" class="grid min-w-0 gap-6 sm:grid-cols-2">
        <div>
          <label for="provider-id" class="mb-2 block text-body font-semibold">
            Provider ID<span class="ml-1 text-danger" aria-hidden="true">*</span>
          </label>
          <input
            id="provider-id"
            v-model="providerID"
            class="atom-field w-full font-mono"
            required
            aria-required="true"
            pattern="[a-z][a-z0-9_-]{0,63}"
            :disabled="!!provider || !!preset"
            placeholder="my-gateway"
          />
          <p class="mt-2 text-note text-label-secondary">小写字母开头；保存后不可修改。</p>
        </div>
        <div>
          <label for="provider-name" class="mb-2 block text-body font-semibold">
            显示名称<span class="ml-1 text-danger" aria-hidden="true">*</span>
          </label>
          <input
            id="provider-name"
            v-model="name"
            class="atom-field w-full"
            required
            aria-required="true"
            maxlength="60"
            placeholder="我的模型网关"
          />
        </div>
        <div class="sm:col-span-2">
          <label for="provider-url" class="mb-2 block text-body font-semibold">
            API 基础地址<span class="ml-1 text-danger" aria-hidden="true">*</span>
          </label>
          <input
            id="provider-url"
            v-model="baseURL"
            class="atom-field w-full font-mono"
            type="url"
            required
            aria-required="true"
            maxlength="2048"
            :disabled="!!catalogID"
            placeholder="https://gateway.example.com/v1"
          />
          <p class="mt-2 text-note text-label-secondary">
            不含 /chat/completions 或 /embeddings。其他端点请添加自定义 API。
          </p>
        </div>
        <div>
          <p class="mb-2 text-body font-semibold">对话 API 协议</p>
          <p v-if="catalogID" class="text-body">OpenAI Chat Completions</p>
          <FilterMenu
            field
            v-else
            :model-value="protocol"
            :options="[
              { key: 'chat', label: 'OpenAI Chat Completions' },
              { key: 'responses', label: 'OpenAI Responses' },
            ]"
            label="选择对话 API 协议"
            @update:model-value="protocol = $event as ModelProtocol"
          />
        </div>
        <div>
          <p class="mb-2 text-body font-semibold">认证方式</p>
          <p v-if="catalogID" class="text-body">API Key</p>
          <FilterMenu
            field
            v-else
            :model-value="auth"
            :options="[
              { key: 'key', label: 'API Key' },
              { key: 'none', label: '无认证（本地服务）' },
            ]"
            label="选择认证方式"
            @update:model-value="changeAuth"
          />
        </div>
        <!-- 高级项：密钥是唯一「只写不回显」的字段，整行铺满并单独分组 -->
        <div v-if="auth === 'key'" class="sm:col-span-2">
          <label
            for="provider-key"
            class="mb-2 flex flex-wrap items-center gap-2 text-body font-semibold"
          >
            <span>API Key ·</span>
            <AppBadge
              size="sm"
              :tone="clearKey ? 'warning' : provider?.credential_configured ? 'primary' : 'neutral'"
              >{{
                clearKey
                  ? '保存后清除'
                  : provider?.credential_configured
                    ? '已配置，不回显'
                    : '尚未配置'
              }}</AppBadge
            >
          </label>
          <div class="flex gap-2">
            <input
              id="provider-key"
              v-model="apiKey"
              class="atom-field min-w-0 flex-1 font-mono"
              type="password"
              autocomplete="new-password"
              maxlength="8192"
              placeholder="留空保留，输入新密钥替换"
            />
            <AppButton class="shrink-0" @click="clearCredential">清除凭据</AppButton>
          </div>
          <p class="mt-2 text-note text-label-secondary">
            密钥仅提交给服务端，不写入浏览器存储。无密钥也可保存为待配置状态。
          </p>
        </div>
      </fieldset>
      <div
        class="mb-4 mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-hairline pt-5"
      >
        <div>
          <h3 class="text-lead">模型目录</h3>
          <p class="mt-1 text-note text-label-secondary">
            Embedding 使用独立的 /embeddings 接口，维度需与模型一致。
          </p>
        </div>
        <AppButton disabled title="后端暂未提供模型发现接口"> 获取可用模型（待接入） </AppButton>
      </div>
      <!-- 模型行列表：行间距 24px（字段 24 / 分节 48） -->
      <fieldset :disabled="busy" class="space-y-6">
        <div
          v-for="(entry, index) in models"
          :key="entry.key"
          class="grid gap-3 bg-surface-secondary p-4 sm:grid-cols-[minmax(0,1fr)_auto_100px_40px]"
        >
          <div>
            <label :for="`model-${entry.key}`" class="mb-2 block text-note font-semibold"
              >模型 ID<span class="ml-1 text-danger" aria-hidden="true">*</span></label
            ><input
              :id="`model-${entry.key}`"
              v-model="entry.model_id"
              class="atom-field w-full"
              required
              aria-required="true"
              maxlength="200"
              placeholder="填写服务商的模型 ID"
            />
          </div>
          <div>
            <p class="mb-2 text-note font-semibold">用途</p>
            <FilterMenu
              field
              :model-value="entry.kind"
              :options="
                catalogID === 'deepseek'
                  ? [{ key: 'chat', label: '对话' }]
                  : [
                      { key: 'chat', label: '对话' },
                      { key: 'embedding', label: 'Embedding' },
                    ]
              "
              :label="`选择模型 ${index + 1} 用途`"
              @update:model-value="changeKind(index, $event)"
            />
          </div>
          <div>
            <label :for="`dimension-${entry.key}`" class="mb-2 block text-note font-semibold"
              >向量维度</label
            ><input
              :id="`dimension-${entry.key}`"
              v-model.number="entry.dimension"
              class="atom-field w-full"
              type="number"
              min="1"
              max="65536"
              step="1"
              :disabled="entry.kind !== 'embedding'"
              :required="entry.kind === 'embedding'"
            />
          </div>
          <AppButton
            tone="quiet"
            icon-only
            icon="material-symbols:close"
            class="sm:mt-6"
            :aria-label="`移除模型 ${index + 1}`"
            @click="models.splice(index, 1)"
          />
        </div>
        <AppButton :disabled="models.length >= 100" @click="addModel"> ＋ 手动添加模型 </AppButton>
      </fieldset>
      <label
        v-if="needsConfirmation"
        class="mt-5 flex items-start gap-3 bg-warning-tint p-4 text-body text-warning-strong"
        ><input
          v-model="reindexConfirmed"
          type="checkbox"
          class="mt-1 accent-primary"
          :disabled="busy"
        />我确认：当前 Embedding 地址或维度变更需要重建向量；保存不会自动重建。</label
      >
      <p v-if="localError || error" role="alert" class="mt-4 text-body text-danger">
        {{ localError || error }}
      </p>
      <AppButton v-if="conflict" class="mt-3" :disabled="busy" @click="reload">
        重新加载最新配置（保留表单，清空密钥）
      </AppButton>
      <footer
        class="mt-6 flex flex-wrap items-center justify-between gap-4 border-t border-hairline pt-5"
      >
        <p class="text-note text-label-secondary">模型调用与连接测试尚未接入。</p>
        <div class="flex gap-2">
          <AppButton disabled>测试连接（待接入）</AppButton>
          <AppButton type="submit" tone="primary" :disabled="busy">
            {{ busy ? '保存中…' : '保存供应商' }}
          </AppButton>
        </div>
      </footer>
    </form>
  </SettingsModal>
</template>
