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
import { agent } from "./agents/agent";
import { startScheduleTool, stopScheduleTool } from "./tools/schedule-tools";
import { plainChatRoute } from "./plain-chat";
import { ragChatRoute } from "./rag-chat";
import { vectorizeRoute } from "./vectorize";
import { conversationRoutes } from "./conversations";
import { healthRoute } from "./health";

export const mastra = new Mastra({
  server: {
    host: "127.0.0.1",
    apiRoutes: [
      plainChatRoute,
      ragChatRoute,
      vectorizeRoute,
      healthRoute,
      ...conversationRoutes,
    ],
  },
  bundler: {
    externals: ["@duckdb/node-bindings"],
  },
  agents: { agent },
  tools: { startScheduleTool, stopScheduleTool },
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
