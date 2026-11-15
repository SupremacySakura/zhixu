import { lookup as dnsLookup } from "node:dns";
import type { LookupAddress, LookupOptions } from "node:dns";
import { AsyncLocalStorage } from "node:async_hooks";
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
//
// A deployment may additionally opt into private model networks for one request.
// That flag arrives from Go with the model configuration and is scoped with
// AsyncLocalStorage, so concurrent turns with different settings never share a
// policy. Link-local, cloud-metadata, multicast and unspecified addresses stay
// blocked even then.

const policyScope = new AsyncLocalStorage<{
  allowPrivateNetworks: boolean;
  /**
   * Plain-HTTP hosts in this request. The lookup hook has no scheme, so
   * dispatch records the host and lookup requires a private answer.
   */
  requirePrivateFor: Set<string>;
}>();

/**
 * Runs `fn` with a request-scoped model egress policy. Async continuations,
 * including undici dispatch and DNS lookup callbacks, inherit the scope.
 */
export function withEgressPolicy<T>(
  allowPrivateNetworks: boolean | undefined,
  fn: () => T,
): T {
  return policyScope.run(
    {
      allowPrivateNetworks: allowPrivateNetworks === true,
      requirePrivateFor: new Set<string>(),
    },
    fn,
  );
}

/**
 * Enters a request-scoped policy for the current async resource. Used by the
 * streaming chat route, whose `start` callback outlives the handler call.
 */
export function enterEgressPolicy(allowPrivateNetworks: boolean | undefined) {
  policyScope.enterWith({
    allowPrivateNetworks: allowPrivateNetworks === true,
    requirePrivateFor: new Set<string>(),
  });
}

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
  /** Deployment opted into private model networks for this request. */
  allowPrivateNetworks: boolean;
  /** Plain-HTTP hosts that must resolve to a private address. */
  requirePrivateFor: Set<string>;
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
 * Addresses that stay blocked even when private model networks are enabled.
 * Link-local reaches cloud metadata and host agents; unspecified, multicast and
 * non-global reserved space are never valid model endpoints.
 */
export function isForbiddenIP(address: string): boolean {
  const family = isIP(address);
  if (family === 4) {
    const [a, b, c] = address.split(".").map(Number);
    if (a === 0) return true; // 0/8
    if (a === 169 && b === 254) return true; // link-local / metadata
    if (a === 192 && b === 0 && c === 0) return true; // 192.0.0/24
    if (a === 198 && (b === 18 || b === 19)) return true; // benchmarking
    if (a >= 224) return true; // multicast, reserved, broadcast
    return address === "100.100.100.200" || address === "168.63.129.16";
  }
  if (family === 6) {
    const groups = ipv6Groups(address);
    if (!groups) return true;
    const mapped = mappedIPv4(groups);
    if (mapped) return isForbiddenIP(mapped);
    const firstGroup = groups[0]!;
    if (groups.every((group) => group === 0)) return true; // unspecified
    if ((firstGroup & 0xffc0) === 0xfe80) return true; // fe80::/10 link-local
    if ((firstGroup & 0xff00) === 0xff00) return true; // ff00::/8 multicast
    if (firstGroup === 0 && !isIPv6Loopback(groups)) return true; // reserved
    return false;
  }
  return true; // anything unparseable is treated as forbidden
}

/**
 * Ranges MODEL_ALLOW_PRIVATE_NETWORKS is meant to open: RFC1918, IPv6
 * unique-local, loopback and Tailscale's CGNAT range. Callers must check
 * isForbiddenIP first because 100.100.100.200 lives inside CGNAT.
 */
export function isPrivateNetworkIP(address: string): boolean {
  const family = isIP(address);
  if (family === 4) {
    if (address === "100.100.100.200" || address === "168.63.129.16")
      return false;
    const [a, b] = address.split(".").map(Number);
    if (a === 10 || a === 127) return true;
    if (a === 172 && b >= 16 && b <= 31) return true;
    if (a === 192 && b === 168) return true;
    if (a === 100 && b >= 64 && b <= 127) return true; // CGNAT / Tailscale
    return false;
  }
  if (family === 6) {
    const groups = ipv6Groups(address);
    if (!groups) return false;
    const mapped = mappedIPv4(groups);
    if (mapped) return isPrivateNetworkIP(mapped);
    if (isIPv6Loopback(groups)) return true;
    return (groups[0]! & 0xfe00) === 0xfc00; // fc00::/7 unique local
  }
  return false;
}

/** Expands any valid IPv6 literal into its eight 16-bit groups. */
function ipv6Groups(address: string): number[] | null {
  let value = address.toLowerCase();
  // Drop an interface scope such as `fe80::1%en0`; DNS answers do not carry one,
  // but configuration might.
  const zone = value.indexOf("%");
  if (zone >= 0) value = value.slice(0, zone);
  if (value.includes(".")) {
    const lastColon = value.lastIndexOf(":");
    const ipv4 = value.slice(lastColon + 1);
    const parts = ipv4.split(".").map(Number);
    if (
      parts.length !== 4 ||
      parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)
    )
      return null;
    const high = ((parts[0]! << 8) | parts[1]!).toString(16);
    const low = ((parts[2]! << 8) | parts[3]!).toString(16);
    value = `${value.slice(0, lastColon + 1)}${high}:${low}`;
  }
  const halves = value.split("::");
  if (halves.length > 2) return null;
  const left = halves[0] ? halves[0].split(":") : [];
  const right = halves.length === 2 && halves[1] ? halves[1].split(":") : [];
  const missing = 8 - left.length - right.length;
  if (halves.length === 1 ? missing !== 0 : missing < 0) return null;
  const groups = [...left, ...Array(missing).fill("0"), ...right].map(
    (group) => (group === "" ? Number.NaN : Number.parseInt(group, 16)),
  );
  if (
    groups.length !== 8 ||
    groups.some(
      (group) => !Number.isInteger(group) || group < 0 || group > 0xffff,
    )
  )
    return null;
  return groups;
}

function mappedIPv4(groups: number[]): string | null {
  if (
    groups[0] !== 0 ||
    groups[1] !== 0 ||
    groups[2] !== 0 ||
    groups[3] !== 0 ||
    groups[4] !== 0 ||
    groups[5] !== 0xffff
  )
    return null;
  return [
    groups[6]! >> 8,
    groups[6]! & 0xff,
    groups[7]! >> 8,
    groups[7]! & 0xff,
  ].join(".");
}

function isIPv6Loopback(groups: number[]): boolean {
  return groups.slice(0, 7).every((group) => group === 0) && groups[7] === 1;
}

/**
 * Builds the allow-list: the exact backend origin plus the hosts the operator
 * listed in EGRESS_ALLOWED_HOSTS.
 */
export function resolveAllowedTargets(
  backendURL: string,
  extraHosts: string,
  allowPrivateNetworks = false,
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
  return {
    origins,
    hosts,
    lookupHosts,
    allowPrivateNetworks,
    requirePrivateFor: new Set<string>(),
  };
}

/**
 * Reads the policy from the environment. Recomputed per call so configuration
 * changes (and tests) never hit a stale cache.
 */
export function currentPolicy(): EgressPolicy {
  const scope = policyScope.getStore();
  const policy = resolveAllowedTargets(
    process.env.CHAT_BACKEND_URL || DEFAULT_CHAT_BACKEND_URL,
    process.env.EGRESS_ALLOWED_HOSTS ?? "",
    scope?.allowPrivateNetworks ?? false,
  );
  if (scope) policy.requirePrivateFor = scope.requirePrivateFor;
  return policy;
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
 * redirect to another port on the same loopback address is rejected. When the
 * request did not opt into private model networks, every non-exempt hop must be
 * HTTPS; only `EGRESS_ALLOWED_HOSTS` may use plain HTTP.
 *
 * When the request did opt in, loopback and private literals are allowed, but
 * forbidden ranges remain blocked. A plain-HTTP hostname is accepted only in
 * that mode, and the lookup hook then requires the name to resolve to a private
 * address, so the switch cannot downgrade a public provider to HTTP.
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
  // A deployment that trusts its private network is a broader grant than the
  // single backend origin, so it must be judged before the exact-origin guard
  // below. Otherwise `CHAT_BACKEND_URL` on loopback would pin every other local
  // port to that origin and the switch would appear to do nothing.
  const isHTTP = url.protocol === "http:";
  const literalIP = isIP(literal) !== 0;
  if (policy.allowPrivateNetworks) {
    if (literalIP) {
      if (isForbiddenIP(literal)) {
        throw denied(hostname, "指向链路本地、云元数据或保留地址");
      }
      if (isPrivateNetworkIP(literal)) return;
    } else if (isHTTP) {
      // The lookup hook requires a private answer for plain-HTTP hosts.
      return;
    }
  }
  // Hosts that only the backend lookup may resolve (the backend hostname) are
  // reachable through their exact origin only. Without this, a redirect to
  // `https://localhost:<other port>` would pass the HTTPS check and the lookup
  // hook would resolve it, because the lookup hook has no port to compare.
  if (policy.lookupHosts.has(hostname) || policy.lookupHosts.has(literal)) {
    throw denied(canonicalOrigin(url), "仅允许精确配置的 origin");
  }
  if (literalIP) {
    if (isForbiddenIP(literal)) {
      throw denied(hostname, "指向链路本地、云元数据或保留地址");
    }
    if (!isUnsafeIP(literal)) {
      if (isHTTP) throw denied(canonicalOrigin(url), "仅允许 HTTPS");
      return;
    }
    throw denied(hostname, "指向私网或保留地址");
  }
  if (isHTTP) throw denied(canonicalOrigin(url), "仅允许 HTTPS");
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
      const policy = currentPolicy();
      if (policy.allowPrivateNetworks && options?.origin) {
        const url = new URL(options.origin);
        if (
          url.protocol === "http:" &&
          !policy.origins.has(canonicalOrigin(url)) &&
          !policy.hosts.has(canonicalHost(url.hostname)) &&
          !policy.hosts.has(stripBrackets(canonicalHost(url.hostname)))
        )
          policy.requirePrivateFor.add(canonicalHost(url.hostname));
      }
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
export function filterLookupAnswers(
  hostname: string,
  list: LookupAddress[],
  options: {
    allowed: Set<string>;
    allowPrivateNetworks: boolean;
    requirePrivateAnswer: boolean;
  },
): LookupAddress[] {
  if (options.allowed.has(canonicalHost(hostname))) return list;
  return list.filter((entry) => {
    if (isForbiddenIP(entry.address)) return false;
    if (options.requirePrivateAnswer) return isPrivateNetworkIP(entry.address);
    return (
      !isUnsafeIP(entry.address) ||
      (options.allowPrivateNetworks && isPrivateNetworkIP(entry.address))
    );
  });
}

export function guardedLookup(
  hostname: string,
  options: LookupOptions,
  callback: LookupCallback,
  allowed: Set<string> = currentPolicy().lookupHosts,
  allowPrivateNetworks = currentPolicy().allowPrivateNetworks,
): void {
  const requirePrivateAnswer = currentPolicy().requirePrivateFor.has(
    canonicalHost(hostname),
  );
  const wantAll = options?.all === true;
  dnsLookup(
    hostname,
    { ...options, all: true, verbatim: true },
    (error, addresses) => {
      if (error) {
        callback(error, [], 0);
        return;
      }
      const safe = filterLookupAnswers(hostname, addresses as LookupAddress[], {
        allowed,
        allowPrivateNetworks,
        requirePrivateAnswer,
      });
      if (safe.length === 0) {
        callback(
          denied(
            hostname,
            requirePrivateAnswer
              ? "明文 HTTP 只允许解析到私网地址"
              : "解析到私网或保留地址",
          ),
          [],
          0,
        );
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
