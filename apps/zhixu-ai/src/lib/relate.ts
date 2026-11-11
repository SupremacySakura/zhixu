import { Agent } from "@mastra/core/agent";
import { z } from "zod";

// Go sends the source knowledge plus a vector-shortlisted candidate set; the AI service
// holds no credentials and only judges relations it can quote from both sides.
export const relateSchema = z
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
    knowledge: z
      .object({
        title: z.string().max(200),
        // Go truncates the adopted summary or the body before sending.
        excerpt: z.string().min(1).max(2_000),
      })
      .strict(),
    // At most ten candidates: one bounded prompt, one bounded model call.
    candidates: z
      .array(
        z
          .object({
            knowledge_id: z.number().int().positive(),
            title: z.string().max(200),
            excerpt: z.string().max(1_200),
            score: z.number(),
          })
          .strict(),
      )
      .min(1)
      .max(10),
  })
  .strict();

export type RelateInput = z.infer<typeof relateSchema>;

export const relationTypeSchema = z.enum([
  "related",
  "depends_on",
  "supplements",
  "contrasts",
  "updates",
]);

export const relateEvidenceSchema = z
  .object({
    // The three fields are what a reader sees before confirming a suggestion.
    reason: z.string().min(1).max(200),
    source_quote: z.string().max(200),
    target_quote: z.string().max(200),
  })
  .strict();

// One suggestion. A malformed entry is dropped on its own; it must not fail the batch.
export const relateItemSchema = z
  .object({
    target_id: z.number().int().positive(),
    relation_type: relationTypeSchema,
    confidence: z.number().min(0).max(1),
    evidence: relateEvidenceSchema,
  })
  .strict();

export type RelateRelation = z.infer<typeof relateItemSchema>;

// Deliberately loose: per-entry validation happens in sanitizeRelations, so one bad
// relation does not throw away the model's other suggestions.
export const relateAnswerSchema = z.object({ relations: z.array(z.unknown()) });

// Schema/parse failures after the internal retry: Go marks the task failed without
// spending further model calls. Transport/provider failures stay retryable on Go.
export class RelateInvalidOutputError extends Error {
  constructor(message = "Model output did not match the relate schema") {
    super(message);
    this.name = "RelateInvalidOutputError";
  }
}

export class RelateUpstreamError extends Error {
  constructor(message = "Model call failed") {
    super(message);
    this.name = "RelateUpstreamError";
  }
}

const INSTRUCTIONS = `你是「知序」个人知识库的关联判定助手。用户会给你一条源知识（标题与摘录）和一批候选知识（每条带候选 ID 与向量相似度）。

请判断源知识与候选之间真实存在的关系，只输出有原文依据的关系：
1. related：主题相关、可互相参照，但不存在方向性的依赖或替代。
2. depends_on：源知识依赖候选知识，缺少候选中的概念源知识就难以成立。
3. supplements：源知识补充候选知识，候选是主体，源提供细节、案例或延伸。
4. contrasts：两者观点、方案或结论相互对比甚至相反。
5. updates：源知识更新或取代候选知识，源更晚、候选已过时。

硬性要求：
- 只能使用给定候选的 ID 作为 target_id，不得编造候选或使用其他 ID。
- confidence 取 0 到 1，表示你对这条关系有原文依据的把握；没有把握就不要输出。
- evidence.reason 用不超过 100 个汉字说明为什么存在这种关系，必须具体，不要写「内容相关」这类空话。
- evidence.source_quote 必须是源摘录中连续出现的一段原文；evidence.target_quote 必须是候选摘录中连续出现的一段原文。两段引文都要能独立支撑该关系，各不超过 200 字，不要改写、不要拼接。
- 最多输出 5 条关系，按 confidence 从高到低排列；宁缺毋滥。
- 不要输出 Markdown、解释或额外字段，只输出符合结构的 JSON。`;

function truncate(value: string, max: number): string {
  const trimmed = value.trim();
  return trimmed.length <= max ? trimmed : trimmed.slice(0, max);
}

// Exported for tests: this prompt is everything the model sees besides the JSON schema.
export function buildRelatePrompt(input: RelateInput): string {
  const lines = [
    `源知识：${truncate(input.knowledge.title, 200) || "（无标题）"}`,
    "源文摘录：",
    input.knowledge.excerpt,
    "",
    "候选知识（只能从中选择 target_id）：",
  ];
  input.candidates.forEach((candidate, index) => {
    lines.push(
      `候选 ${index + 1}（ID ${candidate.knowledge_id}）：${
        truncate(candidate.title, 200) || "（无标题）"
      }（向量相似度 ${candidate.score.toFixed(2)}）`,
      candidate.excerpt,
      "",
    );
  });
  lines.push(
    '输出格式：{"relations":[{"target_id":候选 ID,"relation_type":"related|depends_on|supplements|contrasts|updates","confidence":0 到 1,"evidence":{"reason":"理由","source_quote":"源引文","target_quote":"候选引文"}}]}',
  );
  return lines.join("\n");
}

function trimmed(value: unknown): unknown {
  return typeof value === "string" ? value.trim() : value;
}

// Models pad enums and quotes with whitespace. Trim before validating, then keep only
// entries that are individually valid; dedupe by target and type and cap at five.
export function sanitizeRelations(raw: unknown): RelateRelation[] | null {
  if (typeof raw !== "object" || raw === null) return null;
  const relations = (raw as Record<string, unknown>).relations;
  if (!Array.isArray(relations)) return null;
  const seen = new Set<string>();
  const out: RelateRelation[] = [];
  for (const entry of relations) {
    if (typeof entry !== "object" || entry === null) continue;
    const value = entry as Record<string, unknown>;
    const evidence =
      typeof value.evidence === "object" && value.evidence !== null
        ? (value.evidence as Record<string, unknown>)
        : value.evidence;
    const candidate = {
      target_id: value.target_id,
      relation_type: trimmed(value.relation_type),
      confidence: value.confidence,
      evidence:
        typeof evidence === "object" && evidence !== null
          ? {
              reason: trimmed((evidence as Record<string, unknown>).reason),
              source_quote: trimmed(
                (evidence as Record<string, unknown>).source_quote,
              ),
              target_quote: trimmed(
                (evidence as Record<string, unknown>).target_quote,
              ),
            }
          : evidence,
    };
    const parsed = relateItemSchema.safeParse(candidate);
    if (!parsed.success) continue;
    const key = `${parsed.data.target_id}:${parsed.data.relation_type}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(parsed.data);
    if (out.length === 5) break;
  }
  // A well-formed empty list is a legitimate "no relation"; a non-empty list with no
  // single usable entry is a schema failure worth one internal retry.
  if (out.length === 0 && relations.length > 0) return null;
  return out;
}

// One deterministic structuring call: no tools, no memory, no streaming.
export class RelateAgent {
  async execute(
    input: RelateInput,
    signal: AbortSignal,
  ): Promise<RelateRelation[]> {
    const agent = new Agent({
      id: "relate",
      name: "知识关联判定",
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
      { role: "user" as const, content: buildRelatePrompt(input) },
    ];
    // One automatic retry when the model fails to produce a usable structure.
    for (let attempt = 0; attempt < 2; attempt++) {
      let object: unknown;
      try {
        const result = await agent.generate(messages, {
          structuredOutput: {
            schema: relateAnswerSchema,
            jsonPromptInjection: "auto",
            errorStrategy: "strict",
          },
          modelSettings: { maxRetries: 0 },
          abortSignal: signal,
        });
        object = result?.object;
      } catch (error) {
        if (signal.aborted) throw error;
        if (attempt === 1) throw new RelateUpstreamError();
        continue;
      }
      const relations = sanitizeRelations(object);
      if (relations !== null) return relations;
      if (attempt === 1) throw new RelateInvalidOutputError();
    }
    throw new RelateInvalidOutputError();
  }
}

export const relateAgent = new RelateAgent();
