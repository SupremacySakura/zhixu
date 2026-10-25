import { registerApiRoute } from "@mastra/core/server";
import { z } from "zod";
import { memoryStore } from "../lib/chat-history";
import { authorized } from "../lib/internal-auth";
import { collectHistory, HistoryLimitError } from "../lib/conversation-storage";
import { conversationRuns } from "../lib/conversation-runs";

// Internal conversation-management API. Only the Go backend may call these routes:
// it authenticates end users (JWT + user_id), then forwards the owning resourceId here.
// A shared internal secret proves the caller is the backend, which may assert resourceId.
// Keep the secret private: its disclosure permits impersonating Go and forging user scopes.

const resourceSchema = z
  .object({
    resource: z.string().min(1).max(64),
  })
  .strict();

const renameSchema = resourceSchema.extend({
  title: z.string().min(1).max(200),
});

export const conversationRoutes = [
  registerApiRoute("/internal/conversations", {
    method: "POST",
    handler: async (c) => {
      if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
      const parsed = resourceSchema.safeParse(
        await c.req.json().catch(() => null),
      );
      if (!parsed.success) return c.json({ error: "Invalid input" }, 400);
      const store = await memoryStore();
      let threads;
      try {
        threads = await collectHistory(async (page) => {
          const result = await store.getThreadsByResourceIdPaginated({
            resourceId: parsed.data.resource,
            page,
            perPage: 100,
            orderBy: "updatedAt",
            sortDirection: "DESC",
          });
          return { ...result, items: result.threads };
        });
      } catch (error) {
        if (error instanceof HistoryLimitError)
          return c.json({ error: error.message }, 413);
        throw error;
      }
      return c.json({
        conversations: threads.map((t) => ({
          id: t.id,
          title: t.title,
          metadata: t.metadata ?? null,
          created_at: t.createdAt,
          updated_at: t.updatedAt,
        })),
      });
    },
  }),
  registerApiRoute("/internal/conversations/messages", {
    method: "POST",
    handler: async (c) => {
      if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
      const parsed = resourceSchema
        .extend({ id: z.string().min(1).max(64) })
        .safeParse(await c.req.json().catch(() => null));
      if (!parsed.success) return c.json({ error: "Invalid input" }, 400);
      const store = await memoryStore();
      // Ownership check: the thread must belong to this resource.
      const thread = await store.getThreadById({
        threadId: parsed.data.id,
        resourceId: parsed.data.resource,
      });
      if (!thread) return c.json({ error: "Not found" }, 404);
      let messages;
      try {
        messages = await collectHistory(async (page) => {
          const result = await store.listMessages({
            threadId: parsed.data.id,
            resourceId: parsed.data.resource,
            page,
            perPage: 100,
            orderBy: { field: "createdAt", direction: "ASC" },
          });
          return { ...result, items: result.messages };
        });
      } catch (error) {
        if (error instanceof HistoryLimitError)
          return c.json({ error: error.message }, 413);
        throw error;
      }
      return c.json({
        conversation: {
          id: thread.id,
          title: thread.title,
          metadata: thread.metadata ?? null,
          created_at: thread.createdAt,
          updated_at: thread.updatedAt,
        },
        messages: messages.map((m) => ({
          id: m.id,
          role: m.role,
          content: m.content,
          created_at: m.createdAt,
        })),
      });
    },
  }),
  registerApiRoute("/internal/conversations/rename", {
    method: "POST",
    handler: async (c) => {
      if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
      const parsed = renameSchema
        .extend({ id: z.string().min(1).max(64) })
        .safeParse(await c.req.json().catch(() => null));
      if (!parsed.success) return c.json({ error: "Invalid input" }, 400);
      const store = await memoryStore();
      const thread = await store.getThreadById({
        threadId: parsed.data.id,
        resourceId: parsed.data.resource,
      });
      if (!thread) return c.json({ error: "Not found" }, 404);
      await store.updateThread({
        id: parsed.data.id,
        title: parsed.data.title,
      });
      return c.json({ ok: true });
    },
  }),
  registerApiRoute("/internal/conversations/delete", {
    method: "POST",
    handler: async (c) => {
      if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
      const parsed = resourceSchema
        .extend({ id: z.string().min(1).max(64) })
        .safeParse(await c.req.json().catch(() => null));
      if (!parsed.success) return c.json({ error: "Invalid input" }, 400);
      let found = false;
      // Fence before initialization/ownership awaits: an in-flight first turn may
      // not have created its thread yet. Wait for it before checking and deleting.
      await conversationRuns.delete(parsed.data, async () => {
        const store = await memoryStore();
        const thread = await store.getThreadById({
          threadId: parsed.data.id,
          resourceId: parsed.data.resource,
        });
        if (!thread) return;
        found = true;
        await store.deleteThread({ threadId: parsed.data.id });
      });
      if (!found) return c.json({ error: "Not found" }, 404);
      return c.json({ ok: true });
    },
  }),
];
