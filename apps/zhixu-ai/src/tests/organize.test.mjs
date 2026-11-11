import { test } from "node:test";
import assert from "node:assert/strict";

const {
  buildOrganizePrompt,
  isSchemaValidationError,
  normalizeDraft,
  organizeDraftSchema,
  organizeDraftWireSchema,
  organizeSchema,
} = await import("../lib/organize.ts");

// P3 wire: Go sends the model configuration plus the article body; the AI service
// holds no credentials and never calls back into Go.
const valid = {
  version: 1,
  model: {
    protocol: "chat",
    model_id: "deepseek-chat",
    base_url: "https://api.example.com/v1",
    api_key: "sk-test",
  },
  taskId: 5,
  knowledgeId: 12,
  title: "HNSW 索引原理",
  content: "HNSW 是一种分层可导航小世界图结构，用于近似最近邻检索。",
  existingTags: ["向量数据库"],
};

test("organize schema accepts a Go organize request", () => {
  const parsed = organizeSchema.safeParse(valid);
  assert.equal(parsed.success, true);
  assert.deepEqual(parsed.data, valid);
});

test("organize schema rejects unusable requests", () => {
  const cases = [
    { ...valid, version: 2 },
    { ...valid, taskId: 0 },
    { ...valid, knowledgeId: -1 },
    { ...valid, title: "t".repeat(201) },
    { ...valid, content: "" },
    { ...valid, content: "x".repeat(12_001) },
    { ...valid, existingTags: ["t".repeat(31)] },
    { ...valid, existingTags: Array.from({ length: 51 }, (_, i) => `t${i}`) },
    { ...valid, model: { ...valid.model, base_url: "not-a-url" } },
    { ...valid, model: { ...valid.model, protocol: "openai" } },
    { ...valid, model: { ...valid.model, model_id: "" } },
    // Strict: an unknown field means Go and the AI service disagree on the contract.
    { ...valid, capability: "a".repeat(64) },
    { ...valid, model: { ...valid.model, dimension: 1024 } },
  ];
  for (const input of cases) {
    assert.equal(
      organizeSchema.safeParse(input).success,
      false,
      JSON.stringify(input).slice(0, 80),
    );
  }
});

test("draft schema accepts a normal model answer", () => {
  const draft = {
    summary:
      "HNSW 用分层图结构实现近似最近邻检索，在召回率和延迟之间取得平衡。",
    keywords: ["HNSW", "近似最近邻", "图索引", "向量检索", "分层"],
    suggestedTags: ["向量数据库", "检索算法"],
    concepts: ["HNSW", "KNN", "可导航小世界图"],
  };
  const parsed = organizeDraftSchema.safeParse(draft);
  assert.equal(parsed.success, true);
});

test("draft schema rejects empty or oversized answers", () => {
  const draft = {
    summary: "一段合法摘要，长度足够通过校验。",
    keywords: ["向量检索"],
    suggestedTags: [],
    concepts: [],
  };
  assert.equal(organizeDraftSchema.safeParse(draft).success, true);
  assert.equal(
    organizeDraftSchema.safeParse({ ...draft, summary: "" }).success,
    false,
  );
  assert.equal(
    organizeDraftSchema.safeParse({ ...draft, keywords: [] }).success,
    false,
  );
  assert.equal(
    organizeDraftSchema.safeParse({
      ...draft,
      keywords: Array.from({ length: 11 }, (_, i) => `k${i}`),
    }).success,
    false,
  );
});

test("draft schemas tolerate long English technical terms", () => {
  const draft = {
    summary:
      "创建 React 应用时推荐从全栈框架起步，例如 Next.js App Router、React Router v7 与 Expo。",
    keywords: ["全栈框架", "Next.js App Router", "React Server Components"],
    suggestedTags: ["React", "前端框架"],
    concepts: ["React Server Components"],
  };
  assert.equal(organizeDraftWireSchema.safeParse(draft).success, true);
  assert.equal(organizeDraftSchema.safeParse(draft).success, true);
});

test("wire schema is looser than the product schema on summary length", () => {
  const draft = {
    summary: "长".repeat(260),
    keywords: ["向量检索"],
    suggestedTags: [],
    concepts: [],
  };
  assert.equal(organizeDraftWireSchema.safeParse(draft).success, true);
  assert.equal(organizeDraftSchema.safeParse(draft).success, false);
});

test("normalizeDraft clamps an oversized summary at a sentence boundary", () => {
  const sentence = "这是一句完整的摘要内容，用来验证截断逻辑。";
  const normalized = normalizeDraft({
    summary: sentence.repeat(20),
    keywords: ["React Server Components"],
  });
  assert.ok([...normalized.summary].length <= 200);
  assert.match(normalized.summary, /。$/);
  assert.deepEqual(normalized.keywords, ["React Server Components"]);
});

test("isSchemaValidationError recognizes Mastra structured-output failures", () => {
  assert.equal(
    isSchemaValidationError({
      id: "STRUCTURED_OUTPUT_SCHEMA_VALIDATION_FAILED",
      message: "boom",
    }),
    true,
  );
  assert.equal(
    isSchemaValidationError(
      new Error("Structured output validation failed: keywords.4"),
    ),
    true,
  );
  assert.equal(isSchemaValidationError(new Error("connection reset")), false);
  assert.equal(isSchemaValidationError(null), false);
});

test("normalizeDraft trims, dedupes and caps model output", () => {
  const normalized = normalizeDraft({
    summary: "  摘要前后有空格  ",
    keywords: [" 向量检索 ", "向量检索", "", "HNSW", 3],
    suggestedTags: ["  数据库  ", "数据库"],
    concepts: Array.from({ length: 9 }, (_, i) => `c${i}`),
  });
  assert.equal(normalized.summary, "摘要前后有空格");
  assert.deepEqual(normalized.keywords, ["向量检索", "HNSW"]);
  assert.deepEqual(normalized.suggestedTags, ["数据库"]);
  assert.equal(normalized.concepts.length, 6);
});

test("buildOrganizePrompt carries title, existing tags and body", () => {
  const prompt = buildOrganizePrompt(valid);
  assert.match(prompt, /标题：HNSW 索引原理/);
  assert.match(prompt, /用户已有标签：向量数据库/);
  assert.match(prompt, /分层可导航小世界图结构/);
  const withoutTags = buildOrganizePrompt({ ...valid, existingTags: [] });
  assert.match(withoutTags, /用户还没有标签/);
});
