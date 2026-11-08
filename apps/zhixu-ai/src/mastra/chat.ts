import { Agent } from "@mastra/core/agent";
import { registerApiRoute } from "@mastra/core/server";
import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import {
  allowedSearchURL,
  searchKnowledge,
  type KnowledgeSource,
} from "../lib/rag";
import {
  allowedProviderURL,
  chatTurnSchema,
  type ChatTurnInput,
} from "../lib/chat";
import { chatMemory, memoryStore } from "../lib/chat-history";
import { authorized } from "../lib/internal-auth";
import { persistedAssistantId } from "../lib/conversation-storage";
import { conversationRuns } from "../lib/conversation-runs";
import { DEFAULT_CHAT_BACKEND_URL } from "../lib/egress-guard";

// One agent serves every turn. Retrieval is a tool, not a pipeline step: the model
// decides itself when to search the knowledge base. The system prompt is the "skill"
// that tells it when and how to call search_knowledge.
const SYSTEM_PROMPT = `你是「知序」知识库助手，运行在用户的个人知识库应用「知序」中，帮助用户管理和问答其知识库文档。

你可以使用工具 search_knowledge 在用户的知识库中检索相关文档片段。是否检索、检索几次，完全由你根据问题判断。

## 何时检索
- 问题涉及用户的文档、笔记、资料内容时，必须先调用 search_knowledge 再回答，不要凭记忆或常识直接作答。
- 寒暄、闲聊、情绪表达，或询问你的身份与能力时，不要检索，直接自然友好地回应，可简要介绍自己的能力（基于知识库问答、回答附来源编号引用）。
- 追问中的指代词（这个、上面、它）要结合对话上下文理解；需要检索时，把指代展开为具体名词作为查询词。

## 如何检索
- query 参数是精炼的检索短语：保留专业术语与原文关键词，不超过 80 字。
- 一次检索结果不理想时，可换关键词再次检索；每轮回答最多检索三次。

## 如何回答
1. 基于检索片段回答，在对应内容后标注 [1] [2] 编号引用，编号与片段在工具返回结果中的顺序一致；多次检索的编号按返回顺序连续排列。
2. 片段只能回答一部分时，先用片段作答，再说明哪些部分无法从知识库确认。
3. 检索不到相关内容或检索服务不可用时，如实告知用户（如「知识库中没有找到相关内容」或检索暂时失败），建议换一种问法或稍后再试，不要编造答案。
4. 片段之外的常识性补充，必须明确标注为推测。

通用规则：用与用户相同的语言回答，简洁分点。`;

// Each tool call consumes one step; 8 steps allow up to three retrievals plus the
// final answer, matching the prompt's "最多检索三次" guidance.
const MAX_STEPS = 8;

// Provider-reported token usage for one turn, safe to expose on the wire. The Go BFF
// persists it per assistant message; a missing usage simply yields no `usage` field.
type TurnUsage = {
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
};

function turnUsage(
  value:
    | {
        inputTokens?: number | undefined;
        outputTokens?: number | undefined;
        totalTokens?: number | undefined;
      }
    | undefined,
): TurnUsage | undefined {
  if (!value) return undefined;
  const { inputTokens, outputTokens, totalTokens } = value;
  if (inputTokens == null && outputTokens == null && totalTokens == null)
    return undefined;
  return {
    input_tokens: inputTokens ?? 0,
    output_tokens: outputTokens ?? 0,
    total_tokens: totalTokens ?? (inputTokens ?? 0) + (outputTokens ?? 0),
  };
}

const searchResultSchema = z.object({
  sources: z.array(
    z.object({
      knowledge_id: z.number(),
      title: z.string(),
      section: z.string(),
      content: z.string(),
      score: z.number(),
    }),
  ),
  error: z.string().optional(),
});

// The tool is request-scoped: it closes over this turn's search capability and abort
// signal, so it is created inside the handler and never registered in index.ts.
function createSearchKnowledgeTool(
  tools: ChatTurnInput["tools"],
  signal: AbortSignal,
) {
  return createTool({
    id: "search_knowledge",
    description:
      "在用户知识库中检索与问题相关的文档片段。问题涉及用户的文档、笔记、资料内容时调用；寒暄、闲聊或询问助手能力时不要调用。",
    inputSchema: z.object({
      query: z
        .string()
        .min(1)
        .max(200)
        .describe(
          "精炼的检索短语：结合对话上下文展开指代词，保留专业术语与关键词",
        ),
    }),
    outputSchema: searchResultSchema,
    execute: async ({ query }) => {
      try {
        return { sources: await searchKnowledge(tools, query, signal) };
      } catch {
        // Let the model report the retrieval failure instead of failing the run.
        return { sources: [], error: "检索服务暂不可用" };
      }
    },
  });
}

export const chatRoute = registerApiRoute("/chat", {
  method: "POST",
  handler: async (c) => {
    if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
    const parsed = chatTurnSchema.safeParse(
      await c.req.json().catch(() => null),
    );
    if (!parsed.success) return c.json({ error: "Invalid chat input" }, 400);
    const input = parsed.data;
    // The provider URL is chosen by the backend, never by the browser; still reject
    // embedded credentials so a malformed configuration cannot leak through logs.
    if (!allowedProviderURL(input.model.base_url)) {
      return c.json({ error: "Invalid model URL" }, 403);
    }
    const backend = process.env.CHAT_BACKEND_URL || DEFAULT_CHAT_BACKEND_URL;
    if (!allowedSearchURL(input.tools.search_url, backend)) {
      return c.json({ error: "Invalid search gateway" }, 403);
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
          try {
            output.enqueue(encoder.encode(JSON.stringify(event) + "\n"));
          } catch {
            abort();
          } // Keep draining SDK completion after client cancellation.
        };
        try {
          const agent = new Agent({
            id: "chat",
            name: "知识助手",
            instructions: SYSTEM_PROMPT,
            model: {
              providerId: "openai",
              modelId: input.model.model_id,
              url: input.model.base_url,
              apiKey: input.model.api_key,
              api: input.model.protocol,
            },
            memory: chatMemory,
            tools: {
              search_knowledge: createSearchKnowledgeTool(
                input.tools,
                controller.signal,
              ),
            },
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
          const result = await agent.stream(promptMessages, {
            maxSteps: MAX_STEPS,
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
          let toolMarkupLeak = false;
          let visibleText = "";
          // Citation cards are rebuilt from tool calls: merge chunks across multiple
          // retrievals, dedupe by document section, keep result order (it defines
          // the [1] [2] numbering the model cites).
          const collected: KnowledgeSource[] = [];
          const seen = new Set<string>();
          let lastQuery = "";
          let sourcesSent = false;
          let usage: TurnUsage | undefined;
          const reader = result.fullStream.getReader();
          try {
            for (;;) {
              const { value, done } = await reader.read();
              if (done) break;
              if (value.type === "text-delta") {
                visibleText += value.payload.text;
                // Some gateways don't parse the model's DSML tool-call markup into
                // structured tool_calls, so it leaks into visible text. Detect it
                // early and fail with actionable guidance instead of showing junk.
                if (
                  !toolMarkupLeak &&
                  visibleText.length <= 600 &&
                  visibleText.includes("DSML")
                )
                  toolMarkupLeak = true;
                if (toolMarkupLeak) {
                  failed = true;
                  abort();
                  continue;
                }
                send({ type: "delta", text: value.payload.text });
              }
              if (
                value.type === "tool-call" &&
                value.payload.toolName === "search_knowledge"
              ) {
                const args = value.payload.args as
                  | Record<string, unknown>
                  | undefined;
                if (typeof args?.query === "string") lastQuery = args.query;
              }
              if (
                value.type === "tool-result" &&
                value.payload.toolName === "search_knowledge"
              ) {
                const toolResult = value.payload.result as
                  | { sources?: KnowledgeSource[] }
                  | undefined;
                for (const source of toolResult?.sources ?? []) {
                  const key = `${source.knowledge_id}:${source.section}`;
                  if (seen.has(key)) continue;
                  seen.add(key);
                  collected.push(source);
                }
                send({ type: "sources", query: lastQuery, items: collected });
                sourcesSent = true;
              }
              if (value.type === "step-finish") {
                // Cumulative across steps; the final `finish` below is authoritative.
                usage = turnUsage(value.payload.totalUsage) ?? usage;
              }
              if (value.type === "finish") {
                usage = turnUsage(value.payload.output.usage) ?? usage;
              }
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
              message: toolMarkupLeak
                ? "模型输出了未解析的工具调用标记（DSML）：当前网关未将工具调用解析为结构化 tool_calls。请确认所用服务支持 function calling（DeepSeek 官方 API 支持；自托管或中转服务需启用对应的 tool-call 解析），或更换支持的模型后手动重试"
                : failed
                  ? "模型调用失败，请检查密钥、地址、模型 ID 和额度；若所用模型不支持工具调用（function calling），请更换支持的模型后手动重试"
                  : "模型响应超时或请求已停止",
            });
          } else {
            // Keep the event shape stable when the model never searched (chitchat)
            // or a retrieval returned nothing.
            if (!sourcesSent)
              send({ type: "sources", query: lastQuery, items: collected });
            // Only this run's IDs are eligible, never the injected history.
            const messageId = convo
              ? await persistedAssistantId(
                  await result.response,
                  convo,
                  memoryStore,
                )
              : undefined;
            send({
              type: "done",
              ...(messageId ? { message_id: messageId } : {}),
              ...(usage ? { usage } : {}),
            });
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
