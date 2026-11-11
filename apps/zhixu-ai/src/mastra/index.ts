import { Mastra } from "@mastra/core/mastra";
import { LibSQLStore } from "@mastra/libsql";
import { DuckDBStore } from "@mastra/duckdb";
import { MastraCompositeStore } from "@mastra/core/storage";
import {
  MastraStorageExporter,
  MastraPlatformExporter,
  Observability,
  SensitiveDataFilter,
} from "@mastra/observability";
import { chatRoute } from "./chat";
import { vectorizeRoute } from "./vectorize";
import { organizeRoute } from "./organize";
import { conversationRoutes } from "./conversations";
import { healthRoute } from "./health";
import { installEgressGuard } from "../lib/egress-guard";

// Outbound model/search calls must resolve to public addresses; see egress-guard.
installEgressGuard();

export const mastra = new Mastra({
  server: {
    // MASTRA_HOST=0.0.0.0 is required when the Go backend runs in a container
    // and reaches this service through the host gateway.
    host: process.env.MASTRA_HOST || "127.0.0.1",
    apiRoutes: [
      chatRoute,
      vectorizeRoute,
      organizeRoute,
      healthRoute,
      ...conversationRoutes,
    ],
  },
  bundler: {
    externals: ["@duckdb/node-bindings"],
  },
  storage: new MastraCompositeStore({
    id: "composite-storage",
    default: new LibSQLStore({
      id: "mastra-storage",
      url: process.env.TURSO_DATABASE_URL || "file:./mastra.db",
      authToken: process.env.TURSO_AUTH_TOKEN || undefined,
    }),
    domains: {
      observability: await new DuckDBStore().getStore("observability"),
    },
  }),
  observability: new Observability({
    configs: {
      default: {
        serviceName: "mastra",
        exporters: [new MastraStorageExporter(), new MastraPlatformExporter()],
        spanOutputProcessors: [new SensitiveDataFilter()],
      },
    },
  }),
});
