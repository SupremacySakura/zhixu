import { Agent } from "@mastra/core/agent";
import { z } from "zod";

// Go sends the trigger knowledge plus a bounded candidate set. One call judges duplicate,
// outdated and conflict together: the same excerpt is not paid for twice. The AI service
// holds no credentials and only judges pairs it can quote from both sides.
export const evolveSchema = z
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
    // At most eight candidates: one bounded prompt, one bounded model call.
    candidates: z
      .array(
        z
          .object({
            knowledge_id: z.number().int().positive(),
            title: z.string().max(200),
            excerpt: z.string().max(1_200),
            score: z.number(),
            // Why the candidate is in the batch: an explicit updates relation is a strong
            // signal, a vector neighbour is only a hint.
            origin: z.enum(["update_relation", "vector_neighbor"]),
          })
          .strict(),
      )
      .min(1)
      .max(8),
  })
  .strict();

export type EvolveInput = z.infer<typeof evolveSchema>;

export const evolveTypeSchema = z.enum(["duplicate", "outdated", "conflict"]);

export const evolveSeveritySchema = z.enum(["review", "strong"]);

export const evolveEvidenceSchema = z
  .object({
    // Which side of the pair the quote comes from: the trigger or the candidate.
    knowledge_id: z.number().int().positive(),
    quote: z.string().min(1).max(200),
  })
  .strict();

// One judgement. A malformed entry is dropped on its own; it must not fail the batch.
export const evolveItemSchema = z
  .object({
    target_id: z.number().int().positive(),
    type: evolveTypeSchema,
    reason: z.string().min(1).max(200),
    evidence: z.array(evolveEvidenceSchema).min(1).max(4),
    severity: evolveSeveritySchema,
    // Only conflicts carry a draft: Go stores it and offers "adopt revision".
    revised_title: z.string().max(200).optional(),
    revised_content: z.string().max(20_000).optional(),
  })
  .strict();

export type EvolveJudgement = z.infer<typeof evolveItemSchema>;

// Deliberately loose: per-entry validation happens in sanitizeJudgements, so one bad
// judgement does not throw away the model's other answers.
export const evolveAnswerSchema = z.object({
  judgements: z.array(z.unknown()),
});

// Schema/parse failures after the internal retry: Go marks the task failed without
// spending further model calls. Transport/provider failures stay retryable on Go.
export class EvolveInvalidOutputError extends Error {
  constructor(message = "Model output did not match the evolve schema") {
    super(message);
    this.name = "EvolveInvalidOutputError";
  }
}

export class EvolveUpstreamError extends Error {
  constructor(message = "Model call failed") {
    super(message);
    this.name = "EvolveUpstreamError";
  }
}

const INSTRUCTIONS = `你是「知序」个人知识库的演化判定助手。用户会给你一条本次入库的知识（标题与摘录）和一批候选知识（每条带候选 ID、向量相似度与候选来源）。请逐条判断候选与本次知识之间是否存在需要人工处理的演化关系：

1. duplicate：两条知识说的是同一件事，只是措辞、结构或详略不同。判断关键是信息是否重合，而不是主题是否相同。
2. outdated：候选知识已被本次知识取代——本次知识更晚、更完整或更准确，候选的结论已经过时。
3. conflict：两条知识对同一事实给出互斥的结论，同时采信会让人无所适从（例如一个说可行、一个说不可行）。

硬性要求：
- 只能使用给定候选的 ID 作为 target_id；evidence 里的 knowledge_id 只能是本次知识的 ID 或该候选的 ID。
- evidence 至少一条、最多四条；每条 quote 必须是对应摘录里连续出现的一段原文，不超过 200 字，不要改写、不要拼接、不要跨段。
- reason 用不超过 100 个汉字说明判定依据，必须具体，不要写「内容相似」这类空话。
- severity：引文能直接支撑结论时用 strong，需要读者自行推断时用 review。
- 判定为 conflict 时额外给出 revised_title 与 revised_content：把两条知识里站得住脚的结论合并成一份可取代原文的修订稿，正文不超过 20000 字；无法确定哪一侧正确就不要给修订稿。
- 没有需要处理的候选时输出空数组；宁缺毋滥，不要为了凑数给出结论。
- 不要输出 Markdown、解释或额外字段，只输出符合结构的 JSON。`;

const ORIGIN_LABELS: Record<string, string> = {
  update_relation: "更新关系",
  vector_neighbor: "向量近邻",
};

function truncate(value: string, max: number): string {
  const trimmed = value.trim();
  return trimmed.length <= max ? trimmed : trimmed.slice(0, max);
}

// Exported for tests: this prompt is everything the model sees besides the JSON schema.
export function buildEvolvePrompt(input: EvolveInput): string {
  const lines = [
    `本次知识（ID ${input.knowledgeId}）：${truncate(input.knowledge.title, 200) || "（无标题）"}`,
    "本次知识摘录：",
    input.knowledge.excerpt,
    "",
    "候选知识（只能从中选择 target_id）：",
  ];
  input.candidates.forEach((candidate, index) => {
    // An explicit updates relation carries no similarity: showing "0.00" would read as
    // "unrelated", so those candidates are labelled by origin alone.
    const meta =
      candidate.origin === "update_relation"
        ? `来源 ${ORIGIN_LABELS[candidate.origin]}`
        : `向量相似度 ${candidate.score.toFixed(2)}，来源 ${
            ORIGIN_LABELS[candidate.origin] ?? candidate.origin
          }`;
    lines.push(
      `候选 ${index + 1}（ID ${candidate.knowledge_id}）：${
        truncate(candidate.title, 200) || "（无标题）"
      }（${meta}）`,
      candidate.excerpt,
      "",
    );
  });
  lines.push(
    '输出格式：{"judgements":[{"target_id":候选 ID,"type":"duplicate|outdated|conflict","reason":"理由","evidence":[{"knowledge_id":知识 ID,"quote":"原文引文"}],"severity":"review|strong","revised_title":"仅 conflict 填","revised_content":"仅 conflict 填"}]}',
  );
  return lines.join("\n");
}

function trimmed(value: unknown): unknown {
  return typeof value === "string" ? value.trim() : value;
}

function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}

// Models pad enums and quotes with whitespace. Trim before validating, cut over-long
// quotes and reasons instead of discarding the whole judgement, then keep only entries
// that are individually valid; dedupe by target and type and cap at five.
export function sanitizeJudgements(raw: unknown): EvolveJudgement[] | null {
  if (typeof raw !== "object" || raw === null) return null;
  const list = (raw as Record<string, unknown>).judgements;
  if (!Array.isArray(list)) return null;
  const seen = new Set<string>();
  const out: EvolveJudgement[] = [];
  for (const entry of list) {
    if (typeof entry !== "object" || entry === null) continue;
    const value = entry as Record<string, unknown>;
    const evidence = Array.isArray(value.evidence)
      ? value.evidence.map((item) => {
          if (typeof item !== "object" || item === null) return item;
          const record = item as Record<string, unknown>;
          return {
            knowledge_id: record.knowledge_id,
            quote: truncate(text(record.quote), 200),
          };
        })
      : value.evidence;
    const candidate: Record<string, unknown> = {
      target_id: value.target_id,
      type: trimmed(value.type),
      reason: truncate(text(value.reason), 200),
      evidence,
      severity: trimmed(value.severity),
    };
    // A draft longer than the contract allows is dropped, not cut: half a revision is
    // worse than none, and Go greys out "adopt revision" when the draft is missing.
    const revisedTitle = truncate(text(value.revised_title), 200);
    const revisedContent = text(value.revised_content).trim();
    if (
      revisedTitle !== "" &&
      revisedContent !== "" &&
      revisedContent.length <= 20_000
    ) {
      candidate.revised_title = revisedTitle;
      candidate.revised_content = revisedContent;
    }
    const parsed = evolveItemSchema.safeParse(candidate);
    if (!parsed.success) continue;
    const key = `${parsed.data.target_id}:${parsed.data.type}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(parsed.data);
    if (out.length === 5) break;
  }
  // A well-formed empty list is a legitimate "nothing to review"; a non-empty list with
  // no single usable entry is a schema failure worth one internal retry.
  if (out.length === 0 && list.length > 0) return null;
  return out;
}

// One deterministic structuring call: no tools, no memory, no streaming.
export class EvolveAgent {
  async execute(
    input: EvolveInput,
    signal: AbortSignal,
  ): Promise<EvolveJudgement[]> {
    const agent = new Agent({
      id: "evolve",
      name: "知识演化判定",
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
      { role: "user" as const, content: buildEvolvePrompt(input) },
    ];
    // One automatic retry when the model fails to produce a usable structure.
    for (let attempt = 0; attempt < 2; attempt++) {
      let object: unknown;
      try {
        const result = await agent.generate(messages, {
          structuredOutput: {
            schema: evolveAnswerSchema,
            jsonPromptInjection: "auto",
            errorStrategy: "strict",
          },
          modelSettings: { maxRetries: 0 },
          abortSignal: signal,
        });
        object = result?.object;
      } catch (error) {
        if (signal.aborted) throw error;
        if (attempt === 1) throw new EvolveUpstreamError();
        continue;
      }
      const judgements = sanitizeJudgements(object);
      if (judgements !== null) return judgements;
      if (attempt === 1) throw new EvolveInvalidOutputError();
    }
    throw new EvolveInvalidOutputError();
  }
}

export const evolveAgent = new EvolveAgent();
