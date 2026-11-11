import { Agent } from "@mastra/core/agent";
import { z } from "zod";

// Go sends the model configuration with every organize request: the AI service stores
// no provider credentials. This mirrors the /chat and /vectorize wire contracts.
export const organizeSchema = z
  .object({
    version: z.literal(1),
    model: z
      .object({
        protocol: z.enum(["chat", "responses"]),
        model_id: z.string().min(1).max(200),
        base_url: z.string().url(),
        api_key: z.string(),
      })
      .strict(),
    taskId: z.number().int().positive(),
    knowledgeId: z.number().int().positive(),
    title: z.string().max(200),
    // Go truncates the body before sending; the cap keeps one request bounded.
    content: z.string().min(1).max(12_000),
    existingTags: z.array(z.string().min(1).max(30)).max(50),
  })
  .strict();

export type OrganizeInput = z.infer<typeof organizeSchema>;

// Model-facing schema. Bounds are deliberately loose: an English term such as
// "React Server Components" is 24 characters, so a 20-character keyword limit makes the
// model's otherwise-correct answer fail validation before we can normalize it.
export const organizeDraftWireSchema = z.object({
  summary: z.string().min(10).max(400),
  keywords: z.array(z.string().min(1).max(40)).min(1).max(10),
  suggestedTags: z.array(z.string().min(1).max(30)).max(8),
  concepts: z.array(z.string().min(1).max(30)).max(6),
});

// Product-facing schema after normalization: the draft the user previews must already
// respect what the Go accept endpoint will store.
export const organizeDraftSchema = z.object({
  summary: z.string().min(10).max(200),
  keywords: z.array(z.string().min(1).max(40)).min(1).max(10),
  suggestedTags: z.array(z.string().min(1).max(30)).max(8),
  concepts: z.array(z.string().min(1).max(30)).max(6),
});

export type OrganizeDraft = z.infer<typeof organizeDraftSchema>;

// Schema/parse failures after the internal retry: Go marks the task failed without
// spending further model calls (FR-ORG-02).
export class OrganizeInvalidOutputError extends Error {
  constructor(message = "Model output did not match the organize schema") {
    super(message);
    this.name = "OrganizeInvalidOutputError";
  }
}

// Transport/provider failures stay retryable on the Go side.
export class OrganizeUpstreamError extends Error {
  constructor(message = "Model call failed") {
    super(message);
    this.name = "OrganizeUpstreamError";
  }
}

const INSTRUCTIONS = `你是「知序」个人知识库的整理助手。用户会给你一篇知识条目的标题、正文和用户已有的标签。

请完成四项整理工作：
1. summary：概括这篇知识的核心内容，写成一个完整通顺的句子或两个短句，最多 200 个字符（约 100 到 150 个汉字）。不要出现「本文」「这篇文章」这类空话，不要罗列。
2. keywords：提取 5 个左右最能代表内容的关键词或术语，每个 2 到 20 个字符；英文技术术语可以稍长，但不要超过 40 个字符。用原文中的说法，不要在关键词内部另起句子。
3. suggestedTags：推荐 3 到 6 个分类标签，每个不超过 15 个字。若用户已有标签含义相符，优先直接复用已有标签的原文写法；其余标签要能概括主题，避免过于宽泛的词（如「知识」「内容」）。
4. concepts：列出 3 到 5 个文中提到的核心概念、方法、工具或人物名称，用于后续建立知识关联。

要求：
- 全部使用与正文相同的语言输出。
- 只依据给定正文，不要补充正文没有的信息，不要编造数据或结论。
- 不要输出 Markdown、编号或解释，只输出符合结构的 JSON 字段。`;

const RETRY_INSTRUCTIONS =
  "上一次输出违反了字段长度限制：summary 最多 200 个字符，keywords 每项最多 40 个字符且最多 10 项，只输出符合 schema 的 JSON。";

function truncate(value: string, max: number): string {
  const trimmed = value.trim();
  return trimmed.length <= max ? trimmed : trimmed.slice(0, max);
}

const SUMMARY_MAX_CHARS = 200;
// Prefer cutting a long summary at a sentence boundary instead of mid-word; the lower
// bound keeps the truncated draft long enough to stay useful.
const SUMMARY_TRUNCATE_FLOOR = 100;

export function clampSummary(value: string): string {
  const chars = Array.from(value.trim());
  if (chars.length <= SUMMARY_MAX_CHARS) return chars.join("");
  const head = chars.slice(0, SUMMARY_MAX_CHARS);
  for (let index = head.length - 1; index >= SUMMARY_TRUNCATE_FLOOR; index--) {
    if ("。！？；.!?;".includes(head[index])) {
      return head
        .slice(0, index + 1)
        .join("")
        .trim();
    }
  }
  return head.join("").trim();
}

// Mastra validates the structured-output schema before returning the object, so an
// over-limit field surfaces as a thrown error rather than a parse failure. That is model
// output we cannot use, not an upstream outage: Go must see 422 instead of retrying 502s.
export function isSchemaValidationError(error: unknown): boolean {
  if (typeof error !== "object" || error === null) return false;
  const value = error as { id?: unknown; message?: unknown };
  return (
    (typeof value.id === "string" &&
      value.id.includes("STRUCTURED_OUTPUT_SCHEMA_VALIDATION_FAILED")) ||
    (typeof value.message === "string" &&
      value.message.includes("Structured output validation failed"))
  );
}

// Exported for tests: the prompt shape is the only thing the model sees besides the body.
export function buildOrganizePrompt(input: OrganizeInput): string {
  const tags = input.existingTags.length
    ? input.existingTags.join("、")
    : "（用户还没有标签）";
  return [
    `标题：${truncate(input.title, 200) || "（无标题）"}`,
    `用户已有标签：${tags}`,
    "正文：",
    input.content,
  ].join("\n");
}

// Models routinely pad entries with whitespace or repeat a keyword. Normalizing before
// validation keeps a nearly-correct answer from burning a retry.
export function normalizeDraft(raw: unknown): unknown {
  if (typeof raw !== "object" || raw === null) return raw;
  const value = raw as Record<string, unknown>;
  const strings = (input: unknown, max: number): string[] => {
    if (!Array.isArray(input)) return [];
    const seen = new Set<string>();
    const out: string[] = [];
    for (const item of input) {
      if (typeof item !== "string") continue;
      const trimmed = item.trim();
      if (!trimmed) continue;
      const key = trimmed.toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      out.push(trimmed.slice(0, max));
    }
    return out;
  };
  return {
    summary:
      typeof value.summary === "string"
        ? clampSummary(value.summary)
        : value.summary,
    keywords: strings(value.keywords, 40).slice(0, 10),
    suggestedTags: strings(value.suggestedTags, 30).slice(0, 8),
    concepts: strings(value.concepts, 30).slice(0, 6),
  };
}

// One deterministic structuring call: no tools, no memory, no streaming.
export class OrganizeAgent {
  async execute(
    input: OrganizeInput,
    signal: AbortSignal,
  ): Promise<OrganizeDraft> {
    const agent = new Agent({
      id: "organize",
      name: "知识整理",
      instructions: INSTRUCTIONS,
      model: {
        providerId: "openai",
        modelId: input.model.model_id,
        url: input.model.base_url,
        apiKey: input.model.api_key,
        api: input.model.protocol,
      },
    });
    const messages = [
      { role: "user" as const, content: buildOrganizePrompt(input) },
    ];
    // FR-ORG-02: one automatic retry when the model fails to produce a valid structure.
    for (let attempt = 0; attempt < 2; attempt++) {
      let object: unknown;
      try {
        const result = await agent.generate(messages, {
          structuredOutput: {
            schema: organizeDraftWireSchema,
            jsonPromptInjection: "auto",
            errorStrategy: "strict",
            ...(attempt === 0 ? {} : { instructions: RETRY_INSTRUCTIONS }),
          },
          modelSettings: { maxRetries: 0 },
          abortSignal: signal,
        });
        object = result?.object;
      } catch (error) {
        if (signal.aborted) throw error;
        // A thrown call is either a provider failure or output Mastra rejected against the
        // schema. Retry once, then report the right class so Go retries transport errors
        // but stops early on unusable output.
        if (attempt === 1) {
          throw isSchemaValidationError(error)
            ? new OrganizeInvalidOutputError()
            : new OrganizeUpstreamError();
        }
        continue;
      }
      const parsed = organizeDraftSchema.safeParse(normalizeDraft(object));
      if (parsed.success) return parsed.data;
      if (attempt === 1) throw new OrganizeInvalidOutputError();
    }
    throw new OrganizeInvalidOutputError();
  }
}

export const organizeAgent = new OrganizeAgent();
