import { registerApiRoute } from "@mastra/core/server";
import { allowedProviderURL } from "../lib/chat";
import { authorized } from "../lib/internal-auth";
import {
  EvolveInvalidOutputError,
  evolveAgent,
  evolveSchema,
} from "../lib/evolve";

export const evolveRoute = registerApiRoute("/evolve", {
  method: "POST",
  handler: async (c) => {
    // Same internal bearer as /chat, /vectorize, /organize and /relate.
    if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
    const parsed = evolveSchema.safeParse(await c.req.json().catch(() => null));
    if (!parsed.success) return c.json({ error: "Invalid evolve input" }, 400);
    const input = parsed.data;
    if (!allowedProviderURL(input.model.base_url)) {
      return c.json({ error: "Invalid model URL" }, 403);
    }
    const controller = new AbortController();
    const abort = () => controller.abort();
    c.req.raw.signal.addEventListener("abort", abort, { once: true });
    if (c.req.raw.signal.aborted) abort();
    // Longer than /relate: one call judges three kinds of judgement and may draft a
    // replacement body for conflicts.
    const timer = setTimeout(abort, 120_000);
    try {
      const judgements = await evolveAgent.execute(input, controller.signal);
      c.header("Cache-Control", "no-store");
      return c.json({ judgements });
    } catch (error) {
      if (controller.signal.aborted) {
        return c.json({ code: "timeout", error: "Evolve timed out" }, 504);
      }
      if (error instanceof EvolveInvalidOutputError) {
        // 422 asks Go to fail the task permanently instead of retrying a bad answer.
        return c.json(
          { code: "schema_invalid", error: "Invalid model output" },
          422,
        );
      }
      // Never persist or echo provider responses, article text, or the capability.
      return c.json(
        { error: "Evolve failed; check model configuration and services" },
        502,
      );
    } finally {
      clearTimeout(timer);
      c.req.raw.signal.removeEventListener("abort", abort);
    }
  },
});
