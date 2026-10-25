import { MySQLStore } from "@mastra/mysql";
import { Memory } from "@mastra/memory";
import { initializedMemoryStore } from "./conversation-storage";

// Conversation history lives in the same MySQL instance as the Go backend so that
// threads/messages are the single source of truth. resourceId is the Go user id
// (stringified), which gives per-user isolation for free on every query.
export const chatStore = new MySQLStore({
  id: "chat-history",
  host: process.env.MYSQL_HOST || "127.0.0.1",
  port: Number(process.env.MYSQL_PORT || 3306),
  user: process.env.MYSQL_USER || "root",
  password: process.env.MYSQL_PASSWORD || "",
  database: process.env.MYSQL_DATABASE || "zhixu",
});

export const memoryStore = () => initializedMemoryStore(chatStore);

// Linear history only: semantic recall needs an embedder + vector store that the AI
// service does not own (embeddings go through the Go gateway capability), so disable it.
export const chatMemory = new Memory({
  storage: chatStore,
  options: {
    lastMessages: 40,
    semanticRecall: false,
    // Titles come from the Go backend (first user message, truncated); the AI service
    // must not spend an extra model call on title generation.
    generateTitle: false,
  },
});
