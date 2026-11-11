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

// Draft dimensions are wider than the prompt asks for on purpose: a model that writes a
// 190-character summary or 7 keywords must not fail the whole task.
export const organizeDraftSchema = z.object({
  summary: z.string().min(10).max(200),
  keywords: z.array(z.string().min(1).max(20)).min(1).max(10),
  suggestedTags: z.array(z.string().min(1).max(30)).max(8),
  concepts: z.array(z.string().max(30)).max(6),
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
1. summary：用 100 到 150 个汉字概括这篇知识的核心内容，写成一个完整通顺的句子或两个短句，不要出现「本文」「这篇文章」这类空话，不要罗列。
2. keywords：提取 5 个左右最能代表内容的关键词或术语，每个 2 到 12 个字，用原文中的说法，不要写成句子。
3. suggestedTags：推荐 3 到 6 个分类标签，每个不超过 15 个字。若用户已有标签含义相符，优先直接复用已有标签的原文写法；其余标签要能概括主题，避免过于宽泛的词（如「知识」「内容」）。
4. concepts：列出 3 到 5 个文中提到的核心概念、方法、工具或人物名称，用于后续建立知识关联。

要求：
- 全部使用与正文相同的语言输出。
- 只依据给定正文，不要补充正文没有的信息，不要编造数据或结论。
- 不要输出 Markdown、编号或解释，只输出符合结构的 JSON 字段。`;

function truncate(value: string, max: number): string {
  const trimmed = value.trim();
  return trimmed.length <= max ? trimmed : trimmed.slice(0, max);
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
      typeof value.summary === "string" ? value.summary.trim() : value.summary,
    keywords: strings(value.keywords, 20).slice(0, 10),
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
            schema: organizeDraftSchema,
            jsonPromptInjection: "auto",
            errorStrategy: "strict",
          },
          modelSettings: { maxRetries: 0 },
          abortSignal: signal,
        });
        object = result?.object;
      } catch (error) {
        if (signal.aborted) throw error;
        // A thrown call may be either a provider failure or unparsable output; retry
        // once, then let Go decide whether the class of failure is worth retrying.
        if (attempt === 1) throw new OrganizeUpstreamError();
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
