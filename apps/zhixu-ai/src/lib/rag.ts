import { z } from "zod";

// Search parameters issued by the Go chat handler. The capability authorizes this
// single chat turn against the internal tool gateway; the AI service never sees
// model credentials at rest or the vector database.
export const searchToolsSchema = z
  .object({
    search_url: z.string().url(),
    capability: z.string().regex(/^[a-f0-9]{64}$/),
  })
  .strict();

export type SearchTools = z.infer<typeof searchToolsSchema>;

export interface KnowledgeSource {
  knowledge_id: number;
  // Chunk identity used by the UI to open the exact passage behind a citation.
  chunk_id: number;
  seq: number;
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
      search.pathname === "/internal/tools/v1/search" &&
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
  tools: SearchTools,
  query: string,
  signal: AbortSignal,
): Promise<KnowledgeSource[]> {
  const response = await fetch(tools.search_url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${tools.capability}`,
    },
    body: JSON.stringify({ query }),
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
      Number.isInteger(source.chunk_id) &&
      Number.isInteger(source.seq) &&
      typeof source.title === "string" &&
      typeof source.section === "string" &&
      typeof source.content === "string" &&
      typeof source.score === "number",
  );
}
