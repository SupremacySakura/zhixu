import { Agent } from "@mastra/core/agent";
import { registerApiRoute } from "@mastra/core/server";
import {
  allowedSearchURL,
  ragChatSchema,
  searchKnowledge,
  type KnowledgeSource,
} from "../lib/rag";
import { chatMemory, memoryStore } from "../lib/chat-history";
import { authorized } from "../lib/internal-auth";
import { persistedAssistantId } from "../lib/conversation-storage";
import { conversationRuns } from "../lib/conversation-runs";
import { allowedGateway } from "../lib/vectorize";

// Rewrite the user's latest message into a short search phrase, keeping a little
// conversation context for pronouns like “这个/上面说的”.
const REWRITE_PROMPT = `你是检索查询转写器。把用户最新的问题改写成一条用于向量检索的短查询。
规则：
- 只输出查询本身，一行，不超过 80 字，不要解释、不要加引号。
- 结合会话把指代词（这个、上面、它）展开为具体名词。
- 保留专业术语与原文关键词，可补充一个同义词。
- 问题与文档无关时，原样输出问题。`;

const ANSWER_PROMPT = `你是一个基于知识库回答的助手。
- 只能使用“知识片段”中的事实回答问题；片段之外的常识性补充要明确标注为推测。
- 回答中使用 [1] [2] 这样的编号引用片段，编号与片段顺序一致。
- 片段与问题无关或没有片段时，直接说“知识库中没有找到相关内容”，不要编造。
- 用与提问相同的语言回答，简洁分点。`;

function latestUserMessage(messages: RagMessages): string {
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i]!.role === "user") return messages[i]!.content;
  }
  return "";
}

type RagMessages = { role: "user" | "assistant"; content: string }[];

function buildModel(input: {
  model: string;
  protocol: "chat" | "responses";
  gateway: string;
  capability: string;
}) {
  return {
    providerId: "openai",
    modelId: input.model,
    url: input.gateway,
    apiKey: input.capability,
    api: input.protocol,
  };
}

function formatSources(sources: KnowledgeSource[]): string {
  return sources
    .map(
      (source, index) =>
        `[${index + 1}] 文档《${source.title}》 ${source.section}\n${source.content}`,
    )
    .join("\n\n");
}

export const ragChatRoute = registerApiRoute("/rag-chat", {
  method: "POST",
  handler: async (c) => {
    if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
    const parsed = ragChatSchema.safeParse(
      await c.req.json().catch(() => null),
    );
    if (!parsed.success) return c.json({ error: "Invalid chat input" }, 400);
    const input = parsed.data;
    const backend = process.env.CHAT_BACKEND_URL || "http://127.0.0.1:9000";
    if (!allowedGateway(input.gateway, backend)) {
      return c.json({ error: "Invalid gateway" }, 403);
    }
    if (!allowedSearchURL(input.rag.search_url, backend)) {
      return c.json({ error: "Invalid search gateway" }, 403);
    }
    const controller = new AbortController();
    const finishRun = conversationRuns.register(input.conversation, controller);
    if (!finishRun) return c.json({ error: "Conversation deleted" }, 410);
    const abort = () => controller.abort();
    c.req.raw.signal.addEventListener("abort", abort, { once: true });
    const timeout = setTimeout(abort, 115_000);
    const encoder = new TextEncoder();
    const model = buildModel(input);
    const stream = new ReadableStream<Uint8Array>({
      async start(output) {
        const send = (event: object) => {
          try { output.enqueue(encoder.encode(JSON.stringify(event) + "\n")); }
          catch { abort(); } // Keep draining SDK completion after client cancellation.
        };
        try {
          // Step 1: rewrite the latest question into a retrieval query.
          const rewriter = new Agent({
            id: "rag-rewrite",
            name: "检索转写",
            instructions: REWRITE_PROMPT,
            model,
          });
          const history = input.messages.slice(-7);
          const rewriteMessages = history.map((m) =>
            m.role === "user"
              ? { role: "user" as const, content: m.content }
              : { role: "assistant" as const, content: m.content },
          );
          // The internal gateway always streams SSE, so collect the rewrite text
          // from a stream instead of a one-shot generate() JSON response.
          const rewrite = await rewriter.stream(rewriteMessages, {
            maxSteps: 1,
            abortSignal: controller.signal,
            modelSettings: { maxRetries: 0 },
          });
          let rewritten = "";
          let rewriteFailed = false;
          const rewriteReader = rewrite.fullStream.getReader();
          try {
            for (;;) {
              const { value, done } = await rewriteReader.read();
              if (done) break;
              if (value.type === "text-delta") rewritten += value.payload.text;
              if (value.type === "error") {
                rewriteFailed = true;
                abort();
              }
            }
          } finally {
            await rewriteReader.cancel().catch(() => {});
          }
          if (rewriteFailed || controller.signal.aborted) {
            send({
              type: "error",
              message: "检索转写失败，请检查对话模型配置后手动重试",
            });
            clearTimeout(timeout);
            c.req.raw.signal.removeEventListener("abort", abort);
            try {
              output.close();
            } catch {
              /* Client already cancelled. */
            }
            return;
          }
          const query = (rewritten || latestUserMessage(input.messages))
            .replace(/\s+/g, " ")
            .trim()
            .slice(0, 200);
          // Step 2: retrieve matching knowledge chunks.
          let sources: KnowledgeSource[] = [];
          if (query) {
            sources = await searchKnowledge(
              input.rag,
              query,
              controller.signal,
            );
          }
          send({ type: "sources", query, items: sources });
          // Step 3: answer with citations, or a plain reply when nothing matched.
          const answerer = new Agent({
            id: "rag-answer",
            name: "知识问答",
            instructions:
              sources.length > 0
                ? `${ANSWER_PROMPT}\n\n知识片段：\n${formatSources(sources)}`
                : `${ANSWER_PROMPT}\n\n本次检索没有命中任何知识片段。`,
            model,
            memory: chatMemory,
          });
          const convo = input.conversation;
          // In conversation mode Memory injects prior history, so only the latest
          // user turn is sent as the prompt; otherwise the full thread is the prompt.
          const promptMessages = (
            convo ? input.messages.slice(-1) : input.messages
          ).map((m) =>
            m.role === "user"
              ? { role: "user" as const, content: m.content }
              : { role: "assistant" as const, content: m.content },
          );
          const result = await answerer.stream(promptMessages, {
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
            // Only the answerer's run IDs are eligible, never the rewrite or latest history.
            const messageId = convo
              ? await persistedAssistantId(await result.response, convo, memoryStore)
              : undefined;
            send({ type: "done", ...(messageId ? { message_id: messageId } : {}) });
          }
        } catch {
          if (!controller.signal.aborted)
            send({
              type: "error",
              message:
                "知识库问答失败，请检查 Mastra、检索服务与模型配置后手动重试",
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
