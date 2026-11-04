import { registerHooks } from "node:module";
import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";

const state = { calls: [], saved: new Map(), stream: null, messages: [] };
globalThis.__chatTest = state;
registerHooks({
  resolve(specifier, context, next) {
    const mocks = [
      "@mastra/core/agent",
      "@mastra/core/server",
      "@mastra/mysql",
      "@mastra/memory",
    ];
    if (mocks.includes(specifier))
      return { url: `testmock:${specifier}`, shortCircuit: true };
    try {
      return next(specifier, context);
    } catch (error) {
      if (specifier.startsWith(".") && !specifier.endsWith(".ts"))
        return next(specifier + ".ts", context);
      throw error;
    }
  },
  load(url, context, next) {
    const sources = {
      "testmock:@mastra/core/server":
        "export const registerApiRoute = (path, options) => ({path,...options});",
      "testmock:@mastra/core/agent":
        'export class Agent { constructor(opts) { this.opts=opts; globalThis.__chatTest.calls.push("agent:"+opts.id); } stream(...args) { return globalThis.__chatTest.stream(this.opts,...args); } }',
      "testmock:@mastra/memory":
        "export class Memory { constructor(opts) { this.opts=opts; } }",
      "testmock:@mastra/mysql": `export class MySQLStore {
        async getStore(name) { const s=globalThis.__chatTest; s.calls.push('init:'+name); return {
          async getThreadById({threadId,resourceId}) { s.calls.push('thread'); return {id:threadId,resourceId,title:'test',createdAt:new Date(),updatedAt:new Date()}; },
          async listMessages({page,perPage,resourceId,threadId}) { s.calls.push('page:'+page); return {messages:s.messages.slice(page*perPage,(page+1)*perPage),total:s.messages.length,hasMore:(page+1)*perPage<s.messages.length}; },
          async getThreadsByResourceIdPaginated({page,perPage}) { return {threads:s.messages.slice(page*perPage,(page+1)*perPage),total:s.messages.length,hasMore:(page+1)*perPage<s.messages.length}; },
          async listMessagesById({messageIds}) { s.calls.push('ids:'+messageIds); return {messages:messageIds.map(id=>s.saved.get(id)).filter(Boolean)}; },
          async updateThread() { s.calls.push('rename'); },
          async deleteThread() { s.calls.push('delete'); }
        }; }
      }`,
    };
    if (sources[url])
      return { format: "module", source: sources[url], shortCircuit: true };
    return next(url, context);
  },
});
const { chatRoute } = await import("../mastra/chat.ts");
const { conversationRoutes } = await import("../mastra/conversations.ts");
const {
  collectHistory,
  HistoryLimitError,
  initializedMemoryStore,
  persistedAssistantId,
} = await import("../lib/conversation-storage.ts");
const { ConversationRuns } = await import("../lib/conversation-runs.ts");

function context(body, token = "Bearer test-secret") {
  return {
    req: {
      header: () => token,
      raw: new Request("http://localhost", { method: "POST" }),
      json: async () => {
        state.calls.push("parse");
        return body;
      },
    },
    json: (data, status = 200) => Response.json(data, { status }),
  };
}
const conversation = { id: "thread-1", resource: "user-1" };
// One turn shape now covers every conversation: the model, the tool gateway and
// the transcript travel in the same envelope. Retrieval is a tool the model may call.
const input = {
  version: 1,
  model: {
    protocol: "chat",
    model_id: "fake",
    base_url: "http://127.0.0.1:11434/v1",
    api_key: "test-key",
  },
  messages: [{ role: "user", content: "hello" }],
  conversation,
  tools: {
    search_url: "http://127.0.0.1:9000/internal/tools/v1/search",
    capability: "a".repeat(64),
  },
};
const assistant = (id, c = conversation) => ({
  id,
  role: "assistant",
  threadId: c.id,
  resourceId: c.resource,
  content: { format: 2, parts: [{ type: "text", text: "answer" }] },
  createdAt: new Date(),
});
function fakeStream(opts, messages, options) {
  const id = opts.id === "rag-rewrite" ? "rewrite-id" : "this-turn-id";
  const message = assistant(id);
  const fullStream = new ReadableStream({
    async start(out) {
      out.enqueue({
        type: "text-delta",
        payload: { text: opts.id === "rag-rewrite" ? "" : "answer" },
      });
      // Providers report usage on the terminal finish chunk.
      out.enqueue({
        type: "finish",
        payload: {
          output: {
            usage: { inputTokens: 12, outputTokens: 7, totalTokens: 19 },
          },
        },
      });
      // Emulate awaited SDK onFinish persistence before transport closes.
      await Promise.resolve();
      if (opts.memory && options.memory) state.saved.set(id, message);
      out.close();
    },
  });
  return Promise.resolve({
    fullStream,
    response: Promise.resolve({ dbMessages: [message] }),
    messages: [assistant("other-concurrent-id"), message],
  });
}
beforeEach(() => {
  process.env.INTERNAL_API_SECRET = "test-secret";
  state.calls = [];
  state.saved.clear();
  state.messages = [];
  state.stream = fakeStream;
});

test("all internal routes reject missing/wrong/empty secrets before parse, storage or model", async () => {
  for (const secret of ["test-secret", "", "   "]) {
    process.env.INTERNAL_API_SECRET = secret;
    for (const route of [chatRoute, ...conversationRoutes]) {
      state.calls = [];
      const response = await route.handler(context(null, "Bearer wrong"));
      assert.equal(response.status, 403);
      assert.deepEqual(state.calls, []);
    }
  }
});

test("cold management read initializes store and returns 245 messages, not default 40", async () => {
  state.messages = Array.from({ length: 245 }, (_, i) => assistant("m" + i));
  const response = await conversationRoutes[1].handler(context(conversation));
  assert.equal(response.status, 200);
  const data = await response.json();
  assert.equal(data.messages.length, 245);
  assert.deepEqual(state.calls, [
    "parse",
    "init:memory",
    "thread",
    "page:0",
    "page:1",
    "page:2",
  ]);
});

test("conversation listing also accumulates beyond first 100", async () => {
  state.messages = Array.from({ length: 205 }, (_, i) => ({
    id: "t" + i,
    createdAt: new Date(),
    updatedAt: new Date(),
  }));
  const response = await conversationRoutes[0].handler(
    context({ resource: "user-1" }),
  );
  assert.equal((await response.json()).conversations.length, 205);
});

test("explicit history cap returns 413 rather than a truncated successful history", async () => {
  state.messages = Array.from({ length: 10001 }, (_, i) => assistant("m" + i));
  const response = await conversationRoutes[1].handler(context(conversation));
  assert.equal(response.status, 413);
  await assert.rejects(
    collectHistory(async () => ({ items: [], total: 1, hasMore: true })),
    /no progress/,
  );
  await assert.rejects(
    collectHistory(async () => ({ items: [], total: 10001, hasMore: true })),
    HistoryLimitError,
  );
});

test("initializer awaits getStore and propagates cold-start errors", async () => {
  let ready = false;
  const promise = initializedMemoryStore({
    async getStore(name) {
      assert.equal(name, "memory");
      await Promise.resolve();
      ready = true;
      return {};
    },
  });
  assert.equal(ready, false);
  await promise;
  assert.equal(ready, true);
  await assert.rejects(
    initializedMemoryStore({
      async getStore() {
        throw new Error("DDL failed");
      },
    }),
    /DDL failed/,
  );
});

test("done identifies this persisted answer, not history or rewrite IDs", async () => {
  state.saved.set("other-concurrent-id", assistant("other-concurrent-id"));
  const response = await chatRoute.handler(context(input));
  assert.equal(response.status, 200);
  const events = (await response.text()).trim().split("\n").map(JSON.parse);
  assert.deepEqual(events.at(-1), {
    type: "done",
    message_id: "this-turn-id",
    usage: { input_tokens: 12, output_tokens: 7, total_tokens: 19 },
  });
  assert(state.calls.includes("ids:this-turn-id"));
});

test("chat route fails fast when the model leaks DSML tool-call markup as text", async () => {
  state.stream = async () => ({
    response: Promise.resolve({ dbMessages: [] }),
    fullStream: new ReadableStream({
      start(output) {
        output.enqueue({
          type: "text-delta",
          payload: {
            text: '<｜｜DSML｜｜ calls>\n<｜｜DSML｜｜ invoke name="search_knowledge">',
          },
        });
        output.enqueue({
          type: "text-delta",
          payload: {
            text: '<｜｜DSML｜｜ parameter name="query" string="true">香蕉手机</｜｜DSML｜｜ parameter>',
          },
        });
        output.close();
      },
    }),
  });
  const response = await chatRoute.handler(context(input));
  const events = (await response.text()).trim().split("\n").map(JSON.parse);
  const last = events.at(-1);
  assert.equal(last.type, "error");
  assert.match(last.message, /工具调用/);
  assert(!events.some((e) => e.type === "done"));
  assert(!events.some((e) => e.type === "delta" && e.text.includes("DSML")));
});

test("done waits for SDK stream completion even when response resolves before persistence", async () => {
  let finish;
  state.stream = async () => ({
    response: Promise.resolve({ dbMessages: [assistant("delayed")] }),
    fullStream: new ReadableStream({
      start(output) {
        finish = () => {
          state.saved.set("delayed", assistant("delayed"));
          output.close();
        };
      },
    }),
  });
  const response = await chatRoute.handler(context(input));
  let ended = false;
  const text = response.text().then((value) => {
    ended = true;
    return value;
  });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(ended, false);
  assert(!state.calls.includes("ids:delayed"));
  finish();
  const events = (await text).trim().split("\n").map(JSON.parse);
  assert.deepEqual(events.at(-1), {
    type: "done",
    message_id: "delayed",
  });
});

test("ephemeral chat remains done without claiming a persisted message ID", async () => {
  const { conversation: ignored, ...body } = input;
  const response = await chatRoute.handler(context(body));
  const events = (await response.text()).trim().split("\n").map(JSON.parse);
  assert.deepEqual(events.at(-1), {
    type: "done",
    usage: { input_tokens: 12, output_tokens: 7, total_tokens: 19 },
  });
  assert(!state.calls.includes("init:memory"));
});

test("done omits usage when the provider reports none", async () => {
  state.stream = async () => ({
    response: Promise.resolve({ dbMessages: [] }),
    fullStream: new ReadableStream({
      start(output) {
        output.enqueue({ type: "text-delta", payload: { text: "hi" } });
        output.close();
      },
    }),
  });
  const { conversation: ignored, ...body } = input;
  const response = await chatRoute.handler(context(body));
  const events = (await response.text()).trim().split("\n").map(JSON.parse);
  assert.deepEqual(events.at(-1), { type: "done" });
});

test("cold-start rename initializes before thread access", async () => {
  const response = await conversationRoutes[2].handler(
    context({ ...conversation, title: "renamed" }),
  );
  assert.equal(response.status, 200);
  assert.deepEqual(state.calls, ["parse", "init:memory", "thread", "rename"]);
});

test("missing/foreign/unpersisted IDs cannot generate a successful done", async () => {
  const getStore = async () => ({
    listMessagesById: async () => ({ messages: [] }),
  });
  await assert.rejects(
    persistedAssistantId({ dbMessages: [] }, conversation, getStore),
    /Missing assistant message ID/,
  );
  // Multi-step runs (tool calls) emit several assistant messages; the visible answer is the last one.
  const keepLast = async () => ({
    listMessagesById: async () => ({ messages: [assistant("b")] }),
  });
  assert.equal(
    await persistedAssistantId(
      { dbMessages: [assistant("a"), assistant("b")] },
      conversation,
      keepLast,
    ),
    "b",
  );
  await assert.rejects(
    persistedAssistantId(
      { dbMessages: [assistant("a")] },
      conversation,
      getStore,
    ),
    /not persisted/,
  );
  await assert.rejects(
    persistedAssistantId(
      { dbMessages: [assistant("a")] },
      conversation,
      async () => ({
        listMessagesById: async () => ({
          messages: [assistant("a", { id: "wrong", resource: "user-1" })],
        }),
      }),
    ),
    /not persisted/,
  );
  state.stream = (opts, messages, options) =>
    fakeStream({ ...opts, memory: undefined }, messages, options);
  const response = await chatRoute.handler(context(input));
  const events = (await response.text()).trim().split("\n").map(JSON.parse);
  assert.equal(events.at(-1).type, "error");
  assert(!events.some((e) => e.type === "done"));
});

test("delete route waits for in-flight cold-start stream and blocks new turns before storage", async () => {
  const c = { id: "delete-inflight", resource: "user-1" };
  let finish, started;
  const didStart = new Promise((resolve) => {
    started = resolve;
  });
  state.stream = async (opts, messages, options) => ({
    response: Promise.resolve({ dbMessages: [assistant("inflight", c)] }),
    fullStream: new ReadableStream({
      start(output) {
        started();
        finish = () => {
          state.calls.push("late-save");
          state.saved.set("inflight", assistant("inflight", c));
          output.close();
        };
        options.abortSignal.addEventListener(
          "abort",
          () => state.calls.push("aborted"),
          { once: true },
        );
      },
    }),
  });
  const response = await chatRoute.handler(
    context({ ...input, conversation: c }),
  );
  await didStart;
  const deleting = conversationRoutes[3].handler(context(c));
  // Allow parse continuation to synchronously fence/abort, but not resolve stream.
  await new Promise((resolve) => setImmediate(resolve));
  assert(state.calls.includes("aborted"));
  assert(!state.calls.includes("delete"));
  assert(!state.calls.includes("init:memory"));
  const blocked = await chatRoute.handler(
    context({ ...input, conversation: c }),
  );
  assert.equal(blocked.status, 410);
  finish();
  assert.equal((await deleting).status, 200);
  await response.text();
  assert(state.calls.indexOf("late-save") < state.calls.indexOf("delete"));
});

test("delete aborts all matching runs, fences new runs, waits for settled persistence then deletes", async () => {
  const runs = new ConversationRuns();
  const first = new AbortController(),
    second = new AbortController(),
    other = new AbortController();
  const end1 = runs.register(conversation, first),
    end2 = runs.register(conversation, second);
  const endOther = runs.register(
    { ...conversation, resource: "another-user" },
    other,
  );
  let deleted = false;
  const deleting = runs.delete(conversation, async () => {
    deleted = true;
  });
  assert(first.signal.aborted && second.signal.aborted);
  assert.equal(other.signal.aborted, false);
  assert.equal(runs.register(conversation, new AbortController()), null);
  assert.equal(deleted, false);
  end1();
  await Promise.resolve();
  assert.equal(deleted, false);
  end2();
  await deleting;
  assert.equal(deleted, true);
  assert.equal(runs.register(conversation, new AbortController()), null);
  endOther();
});
