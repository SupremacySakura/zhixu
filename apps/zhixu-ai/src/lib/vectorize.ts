import { ModelRouterEmbeddingModel } from "@mastra/core/llm";
import { z } from "zod";

// The AI service holds no provider credentials: each batch carries the model it must
// use, resolved and decrypted by Go for that task. No gateway indirection is involved.
export const vectorizeSchema = z
  .object({
    model: z.object({
      protocol: z.enum(["chat", "responses"]),
      model_id: z.string().trim().min(1).max(200),
      base_url: z.string().url(),
      api_key: z.string(),
    }),
    dimension: z.number().int().min(1).max(65536),
    // 4096 覆盖「标题 ≤200 字 + 代表文本 1000 字」的最坏 UTF-16 长度，也覆盖长提问的
    // 查询向量化。上限过紧会让整批请求 400（历史上 relation 任务正是这样必失败）。
    texts: z.array(z.string().min(1).max(4096)).min(1).max(16),
  })
  .strict();

export type VectorizeInput = z.infer<typeof vectorizeSchema>;

// Specialized embedding executor: one deterministic SDK call, no chat model, tools, or memory.
export class VectorizeAgent {
  async execute(
    input: VectorizeInput,
    signal: AbortSignal,
  ): Promise<number[][]> {
    const model = new ModelRouterEmbeddingModel({
      providerId: "openai",
      modelId: input.model.model_id,
      url: input.model.base_url,
      apiKey: input.model.api_key,
      api: input.model.protocol,
    });
    const result = await model.doEmbed({
      values: input.texts,
      abortSignal: signal,
    });
    if (
      result.embeddings.length !== input.texts.length ||
      result.embeddings.some(
        (vector) =>
          vector.length !== input.dimension ||
          vector.some((value) => !Number.isFinite(value)) ||
          !vector.some((value) => value !== 0),
      )
    ) {
      throw new Error("Invalid embedding dimensions or values");
    }
    return result.embeddings;
  }
}

export const vectorizeAgent = new VectorizeAgent();
