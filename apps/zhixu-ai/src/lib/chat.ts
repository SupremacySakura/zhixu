import { z } from "zod";

// One chat turn as issued by the Go backend. The AI service calls the model provider
// directly; Go keeps credential storage and issues a per-turn, read-only capability for
// the knowledge-search tool. There is no "rag mode": the model decides when to search.
export const chatTurnSchema = z
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
    messages: z
      .array(
        z.object({
          role: z.enum(["user", "assistant"]),
          content: z.string().min(1).max(8000),
        }),
      )
      .min(1)
      .max(79),
    // Optional conversation identity. When present, turns are persisted to MySQL and
    // prior history is loaded automatically. Absent = ephemeral single-shot chat.
    conversation: z
      .object({
        id: z.string().min(1).max(64),
        resource: z.string().min(1).max(64),
        title: z.string().max(200).optional(),
        metadata: z.record(z.string(), z.unknown()).optional(),
      })
      .strict()
      .optional(),
    tools: z
      .object({
        search_url: z.string().url(),
        capability: z.string().regex(/^[a-f0-9]{64}$/),
      })
      .strict(),
  })
  .strict();

export type ChatTurnInput = z.infer<typeof chatTurnSchema>;

// Provider URLs come from Go's allow-listed model configuration, but still reject
// embedded credentials and fragments, and never downgrade to a non-HTTP scheme.
export function allowedProviderURL(value: string): boolean {
  try {
    const url = new URL(value);
    return (
      (url.protocol === "https:" || url.protocol === "http:") &&
      !url.username &&
      !url.password &&
      !url.hash
    );
  } catch {
    return false;
  }
}
