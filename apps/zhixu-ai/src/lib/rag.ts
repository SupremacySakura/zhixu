import { z } from "zod";

// Search parameters issued by the Go chat handler. The capability authorizes this
// single chat turn against the internal RAG gateway; the AI service never sees
// model credentials or the vector database.
export const ragChatSchema = z
  .object({
    model: z.string().min(1).max(200),
    protocol: z.enum(["chat", "responses"]),
    gateway: z.string().url(),
    capability: z.string().regex(/^[a-f0-9]{64}$/),
    messages: z
      .array(
        z.object({
          role: z.enum(["user", "assistant"]),
          content: z.string().min(1).max(8000),
        }),
      )
      .min(1)
      .max(79),
    rag: z
      .object({
        capability: z.string().regex(/^[a-f0-9]{64}$/),
        search_url: z.string().url(),
        top_k: z.number().int().min(1).max(10),
      })
      .strict(),
    // Optional conversation identity; when present, turns persist to MySQL and history
    // is auto-loaded by Memory. Go sends only the latest user message in that mode.
    conversation: z
      .object({
        id: z.string().min(1).max(64),
        resource: z.string().min(1).max(64),
        title: z.string().max(200).optional(),
        metadata: z.record(z.string(), z.unknown()).optional(),
      })
      .strict()
      .optional(),
  })
  .strict();

export type RagChatInput = z.infer<typeof ragChatSchema>;

export interface KnowledgeSource {
  knowledge_id: number;
  title: string;
  section: string;
  content: string;
  score: number;
}

// The search gateway must point at the configured Go backend, never at an
// arbitrary URL supplied inside the request body.
export function allowedSearchURL(value: string, backendURL: string): boolean {
  try {
    const search = new URL(value);
    const backend = new URL(backendURL);
    return (
      search.origin === backend.origin &&
      search.pathname === "/internal/rag/v1/search" &&
      !search.username &&
      !search.password &&
      !search.search &&
      !search.hash
    );
  } catch {
    return false;
  }
}

export async function searchKnowledge(
  rag: RagChatInput["rag"],
  query: string,
  signal: AbortSignal,
): Promise<KnowledgeSource[]> {
  const response = await fetch(rag.search_url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${rag.capability}`,
    },
    body: JSON.stringify({ query, topK: rag.top_k }),
    signal,
  });
  if (!response.ok) {
    throw new Error("knowledge search unavailable");
  }
  const result = (await response.json()) as {
    sources?: KnowledgeSource[];
  };
  return (result.sources ?? []).filter(
    (source) =>
      Number.isInteger(source.knowledge_id) &&
      typeof source.title === "string" &&
      typeof source.section === "string" &&
      typeof source.content === "string" &&
      typeof source.score === "number",
  );
}
