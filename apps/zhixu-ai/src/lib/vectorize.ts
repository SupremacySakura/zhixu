import { ModelRouterEmbeddingModel } from "@mastra/core/llm";
import { z } from "zod";

export const vectorizeSchema = z
  .object({
    model: z.string().trim().min(1).max(200),
    dimension: z.number().int().min(1).max(65536),
    gateway: z.string().url(),
    capability: z.string().regex(/^[a-f0-9]{64}$/),
    texts: z.array(z.string().min(1).max(1024)).min(1).max(16),
  })
  .strict();

export type VectorizeInput = z.infer<typeof vectorizeSchema>;

export function allowedGateway(value: string, backendURL: string): boolean {
  try {
    const gateway = new URL(value);
    const backend = new URL(backendURL);
    return (
      gateway.origin === backend.origin &&
      gateway.pathname === "/internal/chat/v1" &&
      !gateway.username &&
      !gateway.password &&
      !gateway.search &&
      !gateway.hash
    );
  } catch {
    return false;
  }
}

// Specialized embedding executor: one deterministic SDK call, no chat model, tools, or memory.
export class VectorizeAgent {
  async execute(
    input: VectorizeInput,
    signal: AbortSignal,
  ): Promise<number[][]> {
    const model = new ModelRouterEmbeddingModel({
      providerId: "openai",
      modelId: input.model,
      url: input.gateway,
      apiKey: input.capability,
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
