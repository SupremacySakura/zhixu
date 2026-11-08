import { registerApiRoute } from "@mastra/core/server";
import { allowedProviderURL } from "../lib/chat";
import { authorized } from "../lib/internal-auth";
import { vectorizeAgent, vectorizeSchema } from "../lib/vectorize";

export const vectorizeRoute = registerApiRoute("/vectorize", {
  method: "POST",
  handler: async (c) => {
    // Same internal bearer as /chat: callers can steer outbound model requests, so
    // only Go (which owns end-user authorization) may invoke this route.
    if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
    const parsed = vectorizeSchema.safeParse(
      await c.req.json().catch(() => null),
    );
    if (!parsed.success)
      return c.json({ error: "Invalid vectorize input" }, 400);
    const input = parsed.data;
    if (!allowedProviderURL(input.model.base_url)) {
      return c.json({ error: "Invalid model URL" }, 403);
    }
    const controller = new AbortController();
    const abort = () => controller.abort();
    c.req.raw.signal.addEventListener("abort", abort, { once: true });
    if (c.req.raw.signal.aborted) abort();
    const timer = setTimeout(abort, 100_000);
    try {
      const embeddings = await vectorizeAgent.execute(input, controller.signal);
      c.header("Cache-Control", "no-store");
      return c.json({ embeddings });
    } catch {
      // Never persist or echo provider responses, source text, or the capability.
      return c.json(
        { error: "Embedding failed; check model configuration and services" },
        502,
      );
    } finally {
      clearTimeout(timer);
      c.req.raw.signal.removeEventListener("abort", abort);
    }
  },
});
