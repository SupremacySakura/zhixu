import { test } from "node:test";
import assert from "node:assert/strict";

const {
  buildRelatePrompt,
  relateItemSchema,
  relateSchema,
  sanitizeRelations,
} = await import("../lib/relate.ts");

// P4 wire: Go sends the model configuration plus the source knowledge and at most ten
// vector-shortlisted candidates. The AI service holds no credentials.
const valid = {
  version: 1,
  model: {
    protocol: "chat",
    model_id: "deepseek-chat",
    base_url: "https://api.example.com/v1",
    api_key: "sk-test",
  },
  taskId: 8,
  knowledgeId: 12,
  knowledge: {
    title: "HNSW 索引原理",
    excerpt: "HNSW 是一种分层可导航小世界图结构，用于近似最近邻检索。",
  },
  candidates: [
    {
      knowledge_id: 21,
      title: "向量检索实践",
      excerpt: "用 HNSW 在百万级向量上做低延迟召回。",
      score: 0.82,
    },
    {
      knowledge_id: 22,
      title: "近似最近邻算法",
      excerpt: "比较 IVF、PQ 与 HNSW 的取舍。",
      score: 0.71,
    },
  ],
};

test("relate schema accepts a Go relate request", () => {
  const parsed = relateSchema.safeParse(valid);
  assert.equal(parsed.success, true);
  assert.deepEqual(parsed.data, valid);
});

test("relate schema rejects unusable requests", () => {
  const cases = [
    { ...valid, version: 2 },
    { ...valid, taskId: 0 },
    { ...valid, knowledgeId: -1 },
    { ...valid, knowledge: { ...valid.knowledge, title: "t".repeat(201) } },
    { ...valid, knowledge: { ...valid.knowledge, excerpt: "x".repeat(2001) } },
    { ...valid, knowledge: { ...valid.knowledge, excerpt: "" } },
    { ...valid, knowledge: { ...valid.knowledge, body: "extra" } },
    { ...valid, candidates: [] },
    {
      ...valid,
      candidates: Array.from({ length: 11 }, (_, i) => ({
        knowledge_id: 100 + i,
        title: `候选${i}`,
        excerpt: "摘录",
        score: 0.7,
      })),
    },
    {
      ...valid,
      candidates: [{ ...valid.candidates[0], excerpt: "x".repeat(1201) }],
    },
    {
      ...valid,
      candidates: [{ ...valid.candidates[0], title: "t".repeat(201) }],
    },
    {
      ...valid,
      candidates: [{ ...valid.candidates[0], knowledge_id: 0 }],
    },
    {
      ...valid,
      candidates: [{ ...valid.candidates[0], score: "0.8" }],
    },
    {
      ...valid,
      candidates: [{ ...valid.candidates[0], extra: true }],
    },
    { ...valid, capability: "a".repeat(64) },
    { ...valid, model: { ...valid.model, dimension: 1024 } },
    { ...valid, model: { ...valid.model, base_url: "not-a-url" } },
  ];
  for (const input of cases) {
    assert.equal(
      relateSchema.safeParse(input).success,
      false,
      JSON.stringify(input).slice(0, 120),
    );
  }
});

test("relation item schema accepts the five supported types", () => {
  for (const relationType of [
    "related",
    "depends_on",
    "supplements",
    "contrasts",
    "updates",
  ]) {
    const item = {
      target_id: 21,
      relation_type: relationType,
      confidence: 0.78,
      evidence: {
        reason: "两篇都在讨论 HNSW 的图结构。",
        source_quote: "分层可导航小世界图结构",
        target_quote: "HNSW 在百万级向量上的召回",
      },
    };
    assert.equal(relateItemSchema.safeParse(item).success, true, relationType);
  }
});

test("relation item schema rejects malformed suggestions", () => {
  const item = {
    target_id: 21,
    relation_type: "related",
    confidence: 0.78,
    evidence: {
      reason: "两篇都在讨论 HNSW 的图结构。",
      source_quote: "分层可导航小世界图结构",
      target_quote: "HNSW 在百万级向量上的召回",
    },
  };
  const cases = [
    { ...item, relation_type: "friend" },
    { ...item, target_id: 0 },
    { ...item, confidence: 1.1 },
    { ...item, confidence: -0.01 },
    { ...item, confidence: "0.8" },
    { ...item, evidence: undefined },
    { ...item, evidence: { ...item.evidence, reason: "" } },
    { ...item, evidence: { ...item.evidence, reason: "r".repeat(201) } },
    { ...item, evidence: { ...item.evidence, source_quote: "q".repeat(201) } },
    { ...item, evidence: { ...item.evidence, target_quote: "q".repeat(201) } },
  ];
  for (const input of cases) {
    assert.equal(relateItemSchema.safeParse(input).success, false);
  }
});

test("sanitizeRelations drops invalid entries and caps the batch", () => {
  const good = {
    target_id: 21,
    relation_type: "related",
    confidence: 0.78,
    evidence: { reason: "相关", source_quote: "甲", target_quote: "乙" },
  };
  const invalid = { ...good, relation_type: "friend" };
  const parsed = sanitizeRelations({
    relations: [invalid, good, { ...good, target_id: 22 }, { ...good, target_id: 23 }, { ...good, target_id: 24 }, { ...good, target_id: 25 }, { ...good, target_id: 26 }],
  });
  assert.equal(parsed.length, 5);
  assert.equal(parsed[0].target_id, 21);
  // 单条非法不会让整次判定失败，也不占用五条名额。
  assert.deepEqual(parsed.map((item) => item.target_id), [21, 22, 23, 24, 25]);
});

test("sanitizeRelations trims text and reports an unusable answer", () => {
  const parsed = sanitizeRelations({
    relations: [
      {
        target_id: 21,
        relation_type: " depends_on ",
        confidence: 0.66,
        evidence: {
          reason: "  依赖关系  ",
          source_quote: "  源引文  ",
          target_quote: "  目标引文  ",
        },
      },
    ],
  });
  assert.equal(parsed[0].relation_type, "depends_on");
  assert.equal(parsed[0].evidence.reason, "依赖关系");
  assert.equal(parsed[0].evidence.source_quote, "源引文");

  assert.deepEqual(sanitizeRelations({ relations: [] }), []);
  // 容器本身不合法、或所有条目都不合法：返回 null，交给内部重试后 422。
  assert.equal(sanitizeRelations({ items: [] }), null);
  assert.equal(sanitizeRelations("nope"), null);
  assert.equal(
    sanitizeRelations({ relations: [{ ...valid.candidates[0], target_id: 21 }] }),
    null,
  );
});

test("buildRelatePrompt carries the source and every candidate", () => {
  const prompt = buildRelatePrompt(valid);
  assert.match(prompt, /HNSW 索引原理/);
  assert.match(prompt, /分层可导航小世界图结构/);
  assert.match(prompt, /候选 1（ID 21）/);
  assert.match(prompt, /向量检索实践/);
  assert.match(prompt, /候选 2（ID 22）/);
  assert.match(prompt, /近似最近邻算法/);
  assert.match(prompt, /0\.82/);
  assert.match(prompt, /source_quote/);
});
