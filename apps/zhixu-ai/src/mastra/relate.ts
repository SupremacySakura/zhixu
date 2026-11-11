import { registerApiRoute } from "@mastra/core/server";
import { allowedProviderURL } from "../lib/chat";
import { authorized } from "../lib/internal-auth";
import {
  RelateInvalidOutputError,
  relateAgent,
  relateSchema,
} from "../lib/relate";

export const relateRoute = registerApiRoute("/relate", {
  method: "POST",
  handler: async (c) => {
    // Same internal bearer as /chat, /vectorize and /organize.
    if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
    const parsed = relateSchema.safeParse(await c.req.json().catch(() => null));
    if (!parsed.success) return c.json({ error: "Invalid relate input" }, 400);
    const input = parsed.data;
    if (!allowedProviderURL(input.model.base_url)) {
      return c.json({ error: "Invalid model URL" }, 403);
    }
    const controller = new AbortController();
    const abort = () => controller.abort();
    c.req.raw.signal.addEventListener("abort", abort, { once: true });
    if (c.req.raw.signal.aborted) abort();
    const timer = setTimeout(abort, 90_000);
    try {
      const relations = await relateAgent.execute(input, controller.signal);
      c.header("Cache-Control", "no-store");
      return c.json({ relations });
    } catch (error) {
      if (controller.signal.aborted) {
        return c.json({ code: "timeout", error: "Relate timed out" }, 504);
      }
      if (error instanceof RelateInvalidOutputError) {
        // 422 asks Go to fail the task permanently instead of retrying a bad answer.
        return c.json(
          { code: "schema_invalid", error: "Invalid model output" },
          422,
        );
      }
      // Never persist or echo provider responses, article text, or the capability.
      return c.json(
        { error: "Relate failed; check model configuration and services" },
        502,
      );
    } finally {
      clearTimeout(timer);
      c.req.raw.signal.removeEventListener("abort", abort);
    }
  },
});
