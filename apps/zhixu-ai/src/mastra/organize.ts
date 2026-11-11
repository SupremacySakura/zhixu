import { registerApiRoute } from "@mastra/core/server";
import { allowedProviderURL } from "../lib/chat";
import { authorized } from "../lib/internal-auth";
import {
  OrganizeInvalidOutputError,
  organizeAgent,
  organizeSchema,
} from "../lib/organize";

export const organizeRoute = registerApiRoute("/organize", {
  method: "POST",
  handler: async (c) => {
    // Same internal bearer as /chat and /vectorize: only Go may steer model calls.
    if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
    const parsed = organizeSchema.safeParse(
      await c.req.json().catch(() => null),
    );
    if (!parsed.success)
      return c.json({ error: "Invalid organize input" }, 400);
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
      const draft = await organizeAgent.execute(input, controller.signal);
      c.header("Cache-Control", "no-store");
      return c.json({ draft });
    } catch (error) {
      if (controller.signal.aborted) {
        return c.json({ code: "timeout", error: "Organize timed out" }, 504);
      }
      if (error instanceof OrganizeInvalidOutputError) {
        // 422 asks Go to fail the task permanently instead of retrying a bad answer.
        return c.json(
          { code: "schema_invalid", error: "Invalid model output" },
          422,
        );
      }
      // Never persist or echo provider responses, article text, or the capability.
      return c.json(
        { error: "Organize failed; check model configuration and services" },
        502,
      );
    } finally {
      clearTimeout(timer);
      c.req.raw.signal.removeEventListener("abort", abort);
    }
  },
});
