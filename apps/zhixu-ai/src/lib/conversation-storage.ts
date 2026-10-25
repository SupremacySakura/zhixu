import type { MemoryMySQL } from "@mastra/mysql";
import type { MastraDBMessage } from "@mastra/core/agent";

export type ConversationStore = Pick<MemoryMySQL,
  "getThreadById" | "getThreadsByResourceIdPaginated" | "listMessages" |
  "listMessagesById" | "updateThread" | "deleteThread">;

// getStore initializes the parent adapter and domain (including cold-start DDL).
export async function initializedMemoryStore(storage: {
  getStore: (name: "memory") => Promise<unknown>;
}): Promise<ConversationStore> {
  const store = await storage.getStore("memory");
  if (!store) throw new Error("memory store unavailable");
  return store as ConversationStore;
}

export class HistoryLimitError extends Error {
  constructor() { super("History exceeds the 10000-record safety limit"); }
}

// The current client has no pagination UI. Accumulate pages, never silently truncate.
export async function collectHistory<T>(fetchPage: (page: number) => Promise<{
  items: T[]; total: number; hasMore: boolean;
}>): Promise<T[]> {
  const items: T[] = [];
  for (let page = 0; page < 100; page++) {
    const result = await fetchPage(page);
    if (result.total > 10_000 || items.length + result.items.length > 10_000)
      throw new HistoryLimitError();
    items.push(...result.items);
    if (!result.hasMore) return items;
    if (!result.items.length) throw new Error("History pagination made no progress");
  }
  throw new HistoryLimitError();
}

export async function persistedAssistantId(
  response: { dbMessages?: MastraDBMessage[] },
  conversation: { id: string; resource: string },
  getStore: () => Promise<ConversationStore>,
): Promise<string> {
  // response.dbMessages is this run's output only; result.messages includes memory.
  // Both routes use maxSteps:1. Reject ambiguity rather than guessing the latest ID.
  const assistants = response.dbMessages?.filter((m) => m.role === "assistant") ?? [];
  if (assistants.length !== 1 || !assistants[0]?.id)
    throw new Error("Missing or ambiguous assistant message ID");
  const id = assistants[0].id;
  const store = await getStore();
  const saved = await store.listMessagesById({ messageIds: [id] });
  if (!saved.messages.some((m) => m.id === id && m.role === "assistant" &&
    m.threadId === conversation.id && m.resourceId === conversation.resource))
    throw new Error("Assistant message was not persisted");
  return id;
}
