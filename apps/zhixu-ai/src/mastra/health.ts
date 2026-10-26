import { registerApiRoute } from "@mastra/core/server";
import { authorized } from "../lib/internal-auth";

// Liveness and internal authentication only: never call a model or mutate storage.
export const healthRoute = registerApiRoute("/internal/health", {
  method: "GET",
  handler: async (c) => {
    c.header("Cache-Control", "no-store");
    if (!authorized(c)) return c.json({ error: "Forbidden" }, 403);
    return c.json({ status: "ok" });
  },
});
