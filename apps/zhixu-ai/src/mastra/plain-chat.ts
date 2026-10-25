import { Agent } from "@mastra/core/agent";
import { registerApiRoute } from "@mastra/core/server";
import { z } from "zod";
import { chatMemory, memoryStore } from "../lib/chat-history";
import { authorized } from "../lib/internal-auth";
import { persistedAssistantId } from "../lib/conversation-storage";
import { conversationRuns } from "../lib/conversation-runs";

const inputSchema = z
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
  })
  .strict();

export const plainChatRoute = registerApiRoute("/plain-chat", {
  method: "POST",
  handler: async (c) => {
    if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
    const parsed = inputSchema.safeParse(await c.req.json().catch(() => null));
    if (!parsed.success) return c.json({ error: "Invalid chat input" }, 400);
    const input = parsed.data;
    const gateway = new URL(input.gateway);
    // The AI service only calls the local Go gateway. Never accept an arbitrary provider URL.
    const backend = new URL(
      process.env.CHAT_BACKEND_URL || "http://127.0.0.1:9000",
    );
    if (
      gateway.origin !== backend.origin ||
      gateway.pathname !== "/internal/chat/v1" ||
      gateway.username ||
      gateway.password ||
      gateway.search ||
      gateway.hash
    ) {
      return c.json({ error: "Invalid gateway" }, 403);
    }
    const controller = new AbortController();
    const finishRun = conversationRuns.register(input.conversation, controller);
    if (!finishRun) return c.json({ error: "Conversation deleted" }, 410);
    const abort = () => controller.abort();
    c.req.raw.signal.addEventListener("abort", abort, { once: true });
    const timeout = setTimeout(abort, 115_000);
    const encoder = new TextEncoder();
    const stream = new ReadableStream<Uint8Array>({
      async start(output) {
        const send = (event: object) => {
          try { output.enqueue(encoder.encode(JSON.stringify(event) + "\n")); }
          catch { abort(); } // Keep draining SDK completion after client cancellation.
        };
        try {
          // Standalone agent wired to MySQL memory: persists turns and reloads history
          // when a conversation id is provided by the Go backend.
          const agent = new Agent({
            id: "plain-chat",
            name: "普通对话",
            instructions:
              "你是一个友好的文字对话助手。只根据本次会话回答，不声称访问文件、知识库或调用工具。",
            model: {
              providerId: "openai",
              modelId: input.model,
              url: input.gateway,
              apiKey: input.capability,
              api: input.protocol,
            },
            memory: chatMemory,
          });
          const convo = input.conversation;
          // In conversation mode Memory injects prior history; send only the latest turn.
          const messages = (
            convo ? input.messages.slice(-1) : input.messages
          ).map((m) =>
            m.role === "user"
              ? { role: "user" as const, content: m.content }
              : { role: "assistant" as const, content: m.content },
          );
          const result = await agent.stream(messages, {
            maxSteps: 1,
            abortSignal: controller.signal,
            modelSettings: { maxRetries: 0 },
            ...(convo
              ? {
                  memory: {
                    thread: {
                      id: convo.id,
                      title: convo.title ?? "新对话",
                      metadata: convo.metadata,
                    },
                    resource: convo.resource,
                  },
                }
              : {}),
          });
          let failed = false;
          const reader = result.fullStream.getReader();
          try {
            for (;;) {
              const { value, done } = await reader.read();
              if (done) break;
              if (value.type === "text-delta")
                send({ type: "delta", text: value.payload.text });
              if (value.type === "error") {
                failed = true;
                abort();
              }
            }
          } finally {
            await reader.cancel().catch(() => {});
          }
          if (failed || controller.signal.aborted) {
            send({
              type: "error",
              message: controller.signal.aborted
                ? "模型响应超时或请求已停止"
                : "模型调用失败，请检查密钥、地址、模型 ID 和额度；可手动重试",
            });
          } else {
            // Drain fullStream before this read: SDK awaits memory/onFinish before close.
            // response alone resolves earlier and is not proof of successful persistence.
            const messageId = convo
              ? await persistedAssistantId(await result.response, convo, memoryStore)
              : undefined;
            send({ type: "done", ...(messageId ? { message_id: messageId } : {}) });
          }
        } catch {
          if (!controller.signal.aborted)
            send({
              type: "error",
              message: "模型调用失败，请检查服务与模型配置后手动重试",
            });
        } finally {
          finishRun();
          clearTimeout(timeout);
          c.req.raw.signal.removeEventListener("abort", abort);
          try {
            output.close();
          } catch {
            /* Client already cancelled. */
          }
        }
      },
      cancel() {
        abort();
        clearTimeout(timeout);
      },
    });
    return new Response(stream, {
      headers: {
        "Content-Type": "application/x-ndjson",
        "Cache-Control": "no-store",
      },
    });
  },
});
