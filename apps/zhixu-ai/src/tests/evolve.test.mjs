import { registerHooks } from "node:module";
import { test } from "node:test";
import assert from "node:assert/strict";

// The route module only needs registerApiRoute; the agent itself is never constructed on
// the paths these tests exercise (auth, schema, model URL all fail before execute).
registerHooks({
  resolve(specifier, context, next) {
    if (specifier === "@mastra/core/server")
      return { url: "evolvemock:server", shortCircuit: true };
    try {
      return next(specifier, context);
    } catch (error) {
      if (specifier.startsWith(".") && !specifier.endsWith(".ts"))
        return next(specifier + ".ts", context);
      throw error;
    }
  },
  load(url, context, next) {
    if (url === "evolvemock:server")
      return {
        format: "module",
        shortCircuit: true,
        source: "export const registerApiRoute = (path, options) => ({path,...options});",
      };
    return next(url, context);
  },
});

const { buildEvolvePrompt, evolveItemSchema, evolveSchema, sanitizeJudgements } =
  await import("../lib/evolve.ts");
const { evolveRoute } = await import("../mastra/evolve.ts");

// Go sends the model configuration plus the trigger knowledge and at most eight candidates.
const valid = {
  version: 1,
  model: {
    protocol: "chat",
    model_id: "deepseek-chat",
    base_url: "https://api.example.com/v1",
    api_key: "sk-test",
  },
  taskId: 9,
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
      score: 0.93,
      origin: "vector_neighbor",
    },
    {
      knowledge_id: 22,
      title: "近似最近邻算法",
      excerpt: "比较 IVF、PQ 与 HNSW 的取舍。",
      score: 0.78,
      origin: "update_relation",
    },
  ],
};

function context({ token, body } = {}) {
  const headers = new Headers();
  return {
    req: {
      header: (name) => (name === "authorization" ? token : undefined),
      json: async () => body,
      raw: { signal: new AbortController().signal },
    },
    header: (name, value) => headers.set(name, value),
    json: (data, status = 200) => Response.json(data, { status, headers }),
  };
}

test("evolve schema accepts a Go evolve request", () => {
  const parsed = evolveSchema.safeParse(valid);
  assert.equal(parsed.success, true);
  assert.deepEqual(parsed.data, valid);
});

test("evolve schema rejects unusable requests", () => {
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
      candidates: Array.from({ length: 9 }, (_, i) => ({
        knowledge_id: 100 + i,
        title: `候选${i}`,
        excerpt: "摘录",
        score: 0.7,
        origin: "vector_neighbor",
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
    { ...valid, candidates: [{ ...valid.candidates[0], knowledge_id: 0 }] },
    { ...valid, candidates: [{ ...valid.candidates[0], score: "0.8" }] },
    // origin 必须说明候选从哪来：缺了它 Go 的分派与提示词都没有依据。
    { ...valid, candidates: [{ ...valid.candidates[0], origin: undefined }] },
    { ...valid, candidates: [{ ...valid.candidates[0], origin: "manual" }] },
    { ...valid, candidates: [{ ...valid.candidates[0], extra: true }] },
    { ...valid, capability: "a".repeat(64) },
    { ...valid, model: { ...valid.model, dimension: 1024 } },
    { ...valid, model: { ...valid.model, base_url: "not-a-url" } },
  ];
  for (const input of cases) {
    assert.equal(
      evolveSchema.safeParse(input).success,
      false,
      JSON.stringify(input).slice(0, 120),
    );
  }
});

test("evolve item schema accepts the three judgement types", () => {
  for (const type of ["duplicate", "outdated", "conflict"]) {
    const item = {
      target_id: 21,
      type,
      reason: "两条知识对同一结论给出互斥说法。",
      evidence: [
        { knowledge_id: 12, quote: "分层可导航小世界图结构" },
        { knowledge_id: 21, quote: "HNSW 在百万级向量上的召回" },
      ],
      severity: "strong",
    };
    assert.equal(evolveItemSchema.safeParse(item).success, true, type);
  }
});

test("evolve item schema rejects malformed judgements", () => {
  const item = {
    target_id: 21,
    type: "duplicate",
    reason: "两条知识说的是一件事。",
    evidence: [{ knowledge_id: 12, quote: "分层可导航小世界图结构" }],
    severity: "review",
  };
  const cases = [
    { ...item, type: "friend" },
    { ...item, target_id: 0 },
    { ...item, severity: "urgent" },
    { ...item, reason: "" },
    { ...item, reason: "r".repeat(201) },
    { ...item, evidence: [] },
    { ...item, evidence: [{ knowledge_id: 12, quote: "" }] },
    { ...item, evidence: [{ knowledge_id: 0, quote: "甲" }] },
    { ...item, evidence: [{ knowledge_id: 12, quote: "q".repeat(201) }] },
    {
      ...item,
      evidence: Array.from({ length: 5 }, () => ({
        knowledge_id: 12,
        quote: "甲",
      })),
    },
    { ...item, revised_content: "x".repeat(20_001) },
  ];
  for (const input of cases) {
    assert.equal(evolveItemSchema.safeParse(input).success, false);
  }
});

test("sanitizeJudgements drops invalid entries and caps the batch", () => {
  const good = {
    target_id: 21,
    type: "duplicate",
    reason: "内容重合",
    evidence: [{ knowledge_id: 12, quote: "甲" }],
    severity: "review",
  };
  const parsed = sanitizeJudgements({
    judgements: [
      { ...good, type: "friend" },
      good,
      { ...good, target_id: 22 },
      { ...good, target_id: 23 },
      { ...good, target_id: 24 },
      { ...good, target_id: 25 },
      { ...good, target_id: 26 },
    ],
  });
  assert.equal(parsed.length, 5);
  // 单条非法不会让整次判定失败，也不占用五条名额。
  assert.deepEqual(
    parsed.map((item) => item.target_id),
    [21, 22, 23, 24, 25],
  );
});

test("sanitizeJudgements trims, truncates quotes and reports an unusable answer", () => {
  const parsed = sanitizeJudgements({
    judgements: [
      {
        target_id: 21,
        type: " conflict ",
        reason: `  ${"理".repeat(300)}  `,
        evidence: [{ knowledge_id: 12, quote: `  ${"引".repeat(300)}  ` }],
        severity: " strong ",
      },
    ],
  });
  assert.equal(parsed[0].type, "conflict");
  assert.equal(parsed[0].severity, "strong");
  // 超长理由与引文截断到契约上限，而不是整条丢弃。
  assert.equal(parsed[0].reason.length, 200);
  assert.equal(parsed[0].evidence[0].quote.length, 200);
  assert.equal(parsed[0].evidence[0].knowledge_id, 12);

  assert.deepEqual(sanitizeJudgements({ judgements: [] }), []);
  // 容器本身不合法、或所有条目都不合法：返回 null，交给内部重试后 422。
  assert.equal(sanitizeJudgements({ items: [] }), null);
  assert.equal(sanitizeJudgements("nope"), null);
  assert.equal(
    sanitizeJudgements({ judgements: [{ ...valid.candidates[0], target_id: 21 }] }),
    null,
  );
});

test("sanitizeJudgements keeps a conflict draft only when it is complete", () => {
  const base = {
    target_id: 21,
    type: "conflict",
    reason: "结论互斥",
    evidence: [{ knowledge_id: 12, quote: "甲" }],
    severity: "strong",
  };
  const withDraft = sanitizeJudgements({
    judgements: [
      {
        ...base,
        revised_title: "  合并后的结论  ",
        revised_content: "  以实测为准，两者在 100 万条以上才出现差异。  ",
      },
    ],
  });
  assert.equal(withDraft[0].revised_title, "合并后的结论");
  assert.equal(withDraft[0].revised_content, "以实测为准，两者在 100 万条以上才出现差异。");

  // 只有标题没有正文、或正文超长：丢弃修订稿，提醒降级为「仅提示」。
  const titleOnly = sanitizeJudgements({
    judgements: [{ ...base, revised_title: "只有标题" }],
  });
  assert.equal(titleOnly[0].revised_title, undefined);
  const tooLong = sanitizeJudgements({
    judgements: [
      { ...base, revised_title: "标题", revised_content: "x".repeat(20_001) },
    ],
  });
  assert.equal(tooLong[0].revised_content, undefined);
});

test("buildEvolvePrompt carries the trigger, every candidate and its origin", () => {
  const prompt = buildEvolvePrompt(valid);
  assert.match(prompt, /本次知识（ID 12）/);
  assert.match(prompt, /HNSW 索引原理/);
  assert.match(prompt, /候选 1（ID 21）/);
  assert.match(prompt, /向量检索实践/);
  assert.match(prompt, /0\.93/);
  assert.match(prompt, /向量近邻/);
  assert.match(prompt, /候选 2（ID 22）/);
  assert.match(prompt, /更新关系/);
  assert.match(prompt, /revised_content/);
});

test("evolve route rejects callers without the internal bearer", async () => {
  const previous = process.env.INTERNAL_API_SECRET;
  try {
    process.env.INTERNAL_API_SECRET = "test-secret";
    assert.equal(evolveRoute.path, "/evolve");
    assert.equal(evolveRoute.method, "POST");
    for (const token of [undefined, "Bearer wrong"]) {
      const response = await evolveRoute.handler(
        context({ token, body: valid }),
      );
      assert.equal(response.status, 403);
    }
  } finally {
    if (previous === undefined) delete process.env.INTERNAL_API_SECRET;
    else process.env.INTERNAL_API_SECRET = previous;
  }
});

test("evolve route rejects bad structure and disallowed model URLs", async () => {
  const previous = process.env.INTERNAL_API_SECRET;
  try {
    process.env.INTERNAL_API_SECRET = "test-secret";
    const unauthorizedBody = await evolveRoute.handler(
      context({ token: "Bearer test-secret", body: { version: 1 } }),
    );
    assert.equal(unauthorizedBody.status, 400);

    // 凭证里带 userinfo 的地址不会被放行，避免模型调用被指向内网跳板。
    const disallowed = await evolveRoute.handler(
      context({
        token: "Bearer test-secret",
        body: {
          ...valid,
          model: { ...valid.model, base_url: "https://user:pass@example.com/v1" },
        },
      }),
    );
    assert.equal(disallowed.status, 403);
  } finally {
    if (previous === undefined) delete process.env.INTERNAL_API_SECRET;
    else process.env.INTERNAL_API_SECRET = previous;
  }
});
