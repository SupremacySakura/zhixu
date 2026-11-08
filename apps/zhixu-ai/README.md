# zhixu-ai

Welcome to your new [Mastra](https://mastra.ai) project! We're excited to see what you build.

This starter provides you with a general-purpose Mastra agent that can research current information, manage multi-step tasks, work with local files, run approved shell commands, and create recurring schedules.

## Features

- A local `workspace/` for files and command execution (created under `src/mastra/public/workspace/` when running `mastra dev`)
- Approval gates for file changes, deletions, and shell commands
- Conversation memory, generated thread titles, and task tracking
- Built-in web search and direct web page fetching
- Recurring schedules that persist across restarts
- Local libSQL storage and DuckDB observability, with optional Turso storage
- A bundled Mastra skill that helps coding agents use current Mastra APIs

## Get started

Set your `OPENAI_API_KEY` in `.env` or in your environment, then run:

```shell
pnpm run dev
```

Open [http://localhost:4111](http://localhost:4111) in your browser to access [Mastra Studio](https://mastra.ai/docs/studio/overview).

Select **Agent** in Mastra Studio and try one of these prompts:

- `Get the weather forecast for Austin this weekend.`
- `Create a landing page for a Japanese sakura festival.`
- `Check the SPCX stock price now, then check it every minute.`

The agent asks for approval before it changes files or runs commands. When it creates a schedule, it returns an ID that you can use to pause the schedule.

## Workspace safety

The local filesystem tools stay inside the `workspace/` directory, which resolves relative to the server's working directory (`src/mastra/public/workspace/` during `mastra dev`). Shell commands start in that directory, but `LocalSandbox` does not provide operating-system isolation by default. Review command approvals carefully, and do not expose this template through an unauthenticated public server.

## Running with the zhixu backend

When used by the Go backend (`apps/zhixu-backend`), set these in the environment that starts the service:

- `INTERNAL_API_SECRET`: shared secret with the backend. `/chat`, `/vectorize`, `/internal/conversations`, `/internal/conversations/*`, and `/internal/health` answer `403` unless it is configured and the request carries `Authorization: Bearer <secret>`.
- `MASTRA_HOST`: listen address. Defaults to `127.0.0.1`; set `0.0.0.0` when the backend runs in a container and connects through the host gateway (`host.docker.internal`). Binding `0.0.0.0` exposes every interface, so keep the machine off untrusted networks: the API routes require `INTERNAL_API_SECRET`, but Mastra Studio (dev server UI) does not.
- `CHAT_BACKEND_URL`: origin of the Go backend (defaults to `http://127.0.0.1:9000`, where the backend listens; set `http://127.0.0.1:9001` when it is reached through the compose port mapping). Knowledge-search requests may only target its `/internal/tools/v1/search` path.

### Outbound address guard

Every outbound `fetch` in this process goes through a guarded undici dispatcher. Two checks run before a connection is made:

- Each request origin, including every redirect hop, is validated before dispatch. Literal IP addresses are rejected when they are private, loopback, link-local, CGNAT, multicast, or cloud-metadata addresses. The WHATWG URL parser normalizes alternative IPv4 spellings first, so `2130706433`, `0x7f000001`, `127.1`, and `127.0.0.1.` are all treated as `127.0.0.1` and rejected.
- Hostnames are resolved at connect time and rejected when they resolve to those same ranges, which closes the DNS-rebinding window between validation and connection.

A model base URL therefore cannot be used to reach the local network, even when its hostname passes the Go-side validation.

Two narrow exceptions exist:

- The backend exemption is the **exact `CHAT_BACKEND_URL` origin** (protocol, host, and port). `localhost` is not exempt by itself, so a redirect to another loopback port is still rejected.
- `EGRESS_ALLOWED_HOSTS` (comma-separated) trusts specific hosts (any port) for local model servers. Those hosts may use plain HTTP and may resolve to private addresses; every other hop must be HTTPS.

Literal private IPs need both entries: `MODEL_ALLOWED_PRIVATE_ENDPOINTS` on the Go side and `EGRESS_ALLOWED_HOSTS` here.

## Storage

The default `file:./mastra.db` database stores agent memory, tasks, and schedules locally. To use Turso, set `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN` in `.env`.

Recurring schedules continue to use model tokens until you pause them. Ask the agent to pause a schedule with the ID returned by `start_schedule`.

## Making it yours

- Edit `src/mastra/agents/agent.ts` to change the model, instructions, memory, workspace, or approval policy.
- Edit `src/mastra/tools/` to customize scheduling.
- Edit `src/mastra/index.ts` to change storage and observability.
- Add files or reusable skills under `src/mastra/public/workspace/` for the agent to use during `mastra dev`.

## Learn more

To learn more about Mastra, visit our [documentation](https://mastra.ai/docs/). If you're new to AI agents, check out our [course](https://mastra.ai/learn) and [YouTube videos](https://youtube.com/@mastra-ai). You can also join our [Discord](https://discord.gg/mastra-ai) community to get help and share your projects.

## Deploy to the Mastra platform

The [Mastra platform](https://projects.mastra.ai) provides two products for deploying and managing AI applications built with the Mastra framework. Learn more in the [Mastra platform documentation](https://mastra.ai/docs/mastra-platform/overview).
