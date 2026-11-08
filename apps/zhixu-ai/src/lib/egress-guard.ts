import { lookup as dnsLookup } from "node:dns";
import type { LookupAddress, LookupOptions } from "node:dns";
import { isIP } from "node:net";
import { Agent, setGlobalDispatcher } from "undici";
import type { Dispatcher } from "undici";

// Fallback for the Go backend origin. The chat route (search gateway) and the
// egress allow-list must agree on this value, so both import it from here.
export const DEFAULT_CHAT_BACKEND_URL = "http://127.0.0.1:9000";

let installed = false;

// The AI service makes the outbound model and knowledge-search requests, so it
// has to enforce the address policy itself. Two layers are installed:
//
// 1. `GuardedAgent.dispatch` inspects every request origin before it is
//    dispatched. That covers literal IP addresses (including non-standard
//    spellings such as `2130706433`, which the WHATWG URL parser normalizes to
//    `127.0.0.1`) and every redirect hop, since undici re-dispatches each hop
//    through the same dispatcher.
// 2. `guardedLookup` runs in undici's `connect.lookup` hook: the address it
//    returns is the address that gets connected to, which closes the
//    DNS-rebinding window between validation and connection.
//
// Exceptions are deliberately narrow. The backend exemption matches one exact
// origin (scheme + host + port), so redirecting a model request to another
// loopback port is still rejected. `EGRESS_ALLOWED_HOSTS` entries are trusted
// hosts (any port, HTTP or HTTPS) for local model servers; everything else must
// use HTTPS and resolve to a public address.

type LookupCallback = (
  err: NodeJS.ErrnoException | null,
  address: string | LookupAddress[],
  family?: number,
) => void;

export interface EgressPolicy {
  /** Exact origins (protocol + host + port) that need no further checking. */
  origins: Set<string>;
  /** Operator-listed hosts trusted for plain HTTP and private answers. */
  hosts: Set<string>;
  /** Hostnames allowed to resolve to private addresses in the lookup hook. */
  lookupHosts: Set<string>;
}

/**
 * Returns true for every IPv4/IPv6 address the AI service must never reach.
 * Kept in sync with the Go-side `unsafeModelIP` checks.
 */
export function isUnsafeIP(address: string): boolean {
  const family = isIP(address);
  if (family === 4) {
    const [a, b, c] = address.split(".").map(Number);
    if (a === 0 || a === 10 || a === 127) return true; // 0/8, private, loopback
    if (a === 169 && b === 254) return true; // link-local
    if (a === 172 && b >= 16 && b <= 31) return true; // 172.16/12
    if (a === 192 && b === 168) return true; // 192.168/16
    if (a === 192 && b === 0 && c === 0) return true; // 192.0.0/24
    if (a === 198 && (b === 18 || b === 19)) return true; // benchmarking
    if (a === 100 && b >= 64 && b <= 127) return true; // 100.64/10 CGNAT
    if (a >= 224) return true; // multicast, reserved, broadcast
    return address === "100.100.100.200" || address === "168.63.129.16";
  }
  if (family === 6) {
    const normalized = address.toLowerCase();
    if (normalized === "::" || normalized === "::1") return true; // unspecified, loopback
    const mapped = /^::ffff:(\d{1,3}(?:\.\d{1,3}){3})$/.exec(normalized);
    if (mapped) return isUnsafeIP(mapped[1]); // IPv4-mapped, dotted form
    const firstGroup = Number.parseInt(normalized.split(":")[0] || "0", 16);
    if (Number.isNaN(firstGroup) || firstGroup === 0) return true; // ::/8 reserves IPv4-compatible
    if ((firstGroup & 0xffc0) === 0xfe80) return true; // fe80::/10 link-local
    if ((firstGroup & 0xfe00) === 0xfc00) return true; // fc00::/7 unique local
    if ((firstGroup & 0xff00) === 0xff00) return true; // ff00::/8 multicast
    return false;
  }
  return true; // anything unparseable is treated as unsafe
}

/**
 * Builds the allow-list: the exact backend origin plus the hosts the operator
 * listed in EGRESS_ALLOWED_HOSTS.
 */
export function resolveAllowedTargets(
  backendURL: string,
  extraHosts: string,
): EgressPolicy {
  const origins = new Set<string>();
  const hosts = new Set<string>();
  const lookupHosts = new Set<string>();

  try {
    const url = new URL(backendURL);
    if (url.protocol === "http:" || url.protocol === "https:") {
      origins.add(canonicalOrigin(url));
      lookupHosts.add(canonicalHost(url.hostname));
    }
  } catch {
    // A malformed backend URL leaves the guard strict; the route rejects it too.
  }
  for (const entry of extraHosts.split(",")) {
    const value = entry.trim().toLowerCase();
    if (!value) continue;
    const host = stripBrackets(canonicalHost(value));
    hosts.add(host);
    lookupHosts.add(host);
  }
  return { origins, hosts, lookupHosts };
}

/**
 * Reads the policy from the environment. Recomputed per call so configuration
 * changes (and tests) never hit a stale cache.
 */
export function currentPolicy(): EgressPolicy {
  return resolveAllowedTargets(
    process.env.CHAT_BACKEND_URL || DEFAULT_CHAT_BACKEND_URL,
    process.env.EGRESS_ALLOWED_HOSTS ?? "",
  );
}

function canonicalHost(hostname: string): string {
  return hostname.replace(/\.$/, "").toLowerCase();
}

function canonicalOrigin(url: URL): string {
  return `${url.protocol}//${url.host}`;
}

function stripBrackets(hostname: string): string {
  return hostname.startsWith("[") && hostname.endsWith("]")
    ? hostname.slice(1, -1)
    : hostname;
}

function denied(target: string, reason: string): NodeJS.ErrnoException {
  const error = new Error(
    `出站目的地被拒绝: ${target} ${reason}`,
  ) as NodeJS.ErrnoException;
  error.code = "EACCES";
  return error;
}

function parseOrigin(origin: string | URL | null | undefined): URL {
  if (!origin) throw denied("<empty>", "缺少目标地址");
  if (origin instanceof URL) return origin;
  try {
    return new URL(origin);
  } catch {
    throw denied(String(origin), "不是有效的 URL");
  }
}

/**
 * Validates a request origin. Literal IPs are judged here because they never
 * reach the DNS lookup hook: the WHATWG URL parser normalizes alternative IPv4
 * spellings (decimal, octal, hex, short forms, trailing dot) before this check
 * runs, so `https://2130706433/` is rejected as `127.0.0.1`.
 *
 * The backend exemption is an exact origin match, not a hostname match, so a
 * redirect to another port on the same loopback address is rejected. Every
 * non-exempt hop must be HTTPS; only `EGRESS_ALLOWED_HOSTS` may use plain HTTP.
 */
export function assertSafeOrigin(
  origin: string | URL | null | undefined,
  policy: EgressPolicy = currentPolicy(),
): void {
  const url = parseOrigin(origin);
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw denied(url.protocol, "不是受支持的协议");
  }
  if (policy.origins.has(canonicalOrigin(url))) return;
  const hostname = canonicalHost(url.hostname);
  const literal = stripBrackets(hostname);
  if (policy.hosts.has(hostname) || policy.hosts.has(literal)) return;
  // Hosts that only the backend lookup may resolve (the backend hostname) are
  // reachable through their exact origin only. Without this, a redirect to
  // `https://localhost:<other port>` would pass the HTTPS check and the lookup
  // hook would resolve it, because the lookup hook has no port to compare.
  if (policy.lookupHosts.has(hostname) || policy.lookupHosts.has(literal)) {
    throw denied(canonicalOrigin(url), "仅允许精确配置的 origin");
  }
  if (url.protocol !== "https:") {
    throw denied(canonicalOrigin(url), "仅允许 HTTPS");
  }
  if (isIP(literal) !== 0 && isUnsafeIP(literal)) {
    throw denied(hostname, "指向私网或保留地址");
  }
}

/**
 * undici handler failure path. Handlers are either old-style (onError) or
 * new-style (onResponseError); replicate what DispatcherBase does.
 */
function rejectHandler(
  handler: Dispatcher.DispatchHandler,
  error: Error,
): boolean {
  if (typeof handler.onError === "function") {
    handler.onError(error);
    return false;
  }
  if (typeof handler.onResponseError === "function") {
    handler.onResponseError(null as never, error);
    return false;
  }
  throw error;
}

/**
 * Dispatcher that applies `assertSafeOrigin` to every request, including each
 * redirect hop followed by `fetch`.
 */
export class GuardedAgent extends Agent {
  dispatch(
    options: Agent.DispatchOptions,
    handler: Dispatcher.DispatchHandler,
  ): boolean {
    try {
      assertSafeOrigin(options?.origin);
    } catch (error) {
      return rejectHandler(handler, error as Error);
    }
    return super.dispatch(options, handler);
  }
}

/**
 * undici `connect.lookup` hook: resolve, reject unsafe answers, and hand back
 * only validated addresses so the connect target is the validated target.
 */
export function guardedLookup(
  hostname: string,
  options: LookupOptions,
  callback: LookupCallback,
  allowed: Set<string> = currentPolicy().lookupHosts,
): void {
  const wantAll = options?.all === true;
  dnsLookup(
    hostname,
    { ...options, all: true, verbatim: true },
    (error, addresses) => {
      if (error) {
        callback(error, [], 0);
        return;
      }
      const list = addresses as LookupAddress[];
      const safe = allowed.has(canonicalHost(hostname))
        ? list
        : list.filter((entry) => !isUnsafeIP(entry.address));
      if (safe.length === 0) {
        callback(denied(hostname, "解析到私网或保留地址"), [], 0);
        return;
      }
      if (wantAll) {
        callback(null, safe);
        return;
      }
      const [first] = safe;
      callback(null, first.address, first.family);
    },
  );
}

/**
 * Routes every outbound `fetch` in this process (model calls, knowledge
 * search) through the guarded dispatcher. Call once during startup.
 */
export function installEgressGuard(): void {
  if (installed) return;
  installed = true;
  setGlobalDispatcher(new GuardedAgent({ connect: { lookup: guardedLookup } }));
}
