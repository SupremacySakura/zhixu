import { test } from "node:test";
import assert from "node:assert/strict";

const { vectorizeSchema } = await import("../lib/vectorize.ts");

// P2 wire: the AI service receives the embedding model directly and never a gateway
// capability. These assertions pin the contract Go sends.
const valid = {
  model: {
    protocol: "chat",
    model_id: "bge-m3",
    base_url: "https://api.example.com/v1",
    api_key: "sk-test",
  },
  dimension: 1024,
  texts: ["第一段", "第二段"],
};

test("vectorize schema accepts a direct provider batch", () => {
  const parsed = vectorizeSchema.safeParse(valid);
  assert.equal(parsed.success, true);
  assert.deepEqual(parsed.data, valid);
});

test("vectorize schema rejects the retired gateway indirection", () => {
  const withGateway = {
    ...valid,
    gateway: "http://127.0.0.1:9000/internal/chat/v1",
    capability: "a".repeat(64),
  };
  assert.equal(vectorizeSchema.safeParse(withGateway).success, false);

  const legacy = {
    model: "bge-m3",
    dimension: 1024,
    texts: ["一段"],
    gateway: "http://127.0.0.1:9000/internal/chat/v1",
    capability: "a".repeat(64),
  };
  assert.equal(vectorizeSchema.safeParse(legacy).success, false);
});

test("vectorize schema rejects unusable model configuration and batches", () => {
  const cases = [
    { ...valid, model: { ...valid.model, model_id: " " } },
    { ...valid, model: { ...valid.model, base_url: "not-a-url" } },
    { ...valid, model: { ...valid.model, protocol: "openai" } },
    { ...valid, dimension: 0 },
    { ...valid, texts: [] },
    { ...valid, texts: [""] },
    { ...valid, texts: Array.from({ length: 17 }, (_, i) => `t${i}`) },
  ];
  for (const input of cases) {
    assert.equal(vectorizeSchema.safeParse(input).success, false);
  }
});
