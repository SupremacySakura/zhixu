import { createServer } from "node:http";
import { test } from "node:test";
import assert from "node:assert/strict";

// Keep the default policy strict: the allow-list must not contain loopback.
process.env.CHAT_BACKEND_URL = "http://backend.internal:9000";
process.env.EGRESS_ALLOWED_HOSTS = "";

const {
  GuardedAgent,
  assertSafeOrigin,
  guardedLookup,
  installEgressGuard,
  isUnsafeIP,
  resolveAllowedTargets,
} = await import("../lib/egress-guard.ts");

test("isUnsafeIP rejects private and reserved IPv4 space", () => {
  for (const address of [
    "0.0.0.0",
    "10.1.2.3",
    "127.0.0.1",
    "169.254.169.254",
    "172.16.0.1",
    "172.31.255.255",
    "192.168.1.1",
    "192.0.0.8",
    "198.18.0.1",
    "198.19.255.255",
    "100.64.0.1",
    "100.127.255.255",
    "100.100.100.200",
    "168.63.129.16",
    "224.0.0.1",
    "255.255.255.255",
  ]) {
    assert.equal(isUnsafeIP(address), true, address);
  }
});

test("isUnsafeIP allows public IPv4 space", () => {
  for (const address of [
    "1.1.1.1",
    "8.8.8.8",
    "100.63.255.255",
    "172.32.0.1",
    "198.20.0.1",
    "203.0.113.10",
  ]) {
    assert.equal(isUnsafeIP(address), false, address);
  }
});

test("isUnsafeIP rejects private and reserved IPv6 space", () => {
  for (const address of [
    "::",
    "::1",
    "0:0:0:0:0:0:0:1",
    "fe80::1",
    "fd00::1",
    "fcff::1",
    "ff02::1",
    "::ffff:127.0.0.1",
    "::ffff:10.0.0.1",
  ]) {
    assert.equal(isUnsafeIP(address), true, address);
  }
});

test("isUnsafeIP allows public IPv6 space", () => {
  for (const address of [
    "2606:4700:4700::1111",
    "2001:4860:4860::8888",
    "::ffff:8.8.8.8",
  ]) {
    assert.equal(isUnsafeIP(address), false, address);
  }
});

test("isUnsafeIP treats unparseable input as unsafe", () => {
  for (const address of ["", "not-an-ip", "10.0.0", "localhost"]) {
    assert.equal(isUnsafeIP(address), true, address);
  }
});

test("resolveAllowedTargets limits the backend exemption to one exact origin", () => {
  const policy = resolveAllowedTargets("http://127.0.0.1:9000", "");
  assert.equal(policy.origins.has("http://127.0.0.1:9000"), true);
  assert.equal(policy.hosts.size, 0);
  assert.equal(policy.lookupHosts.has("127.0.0.1"), true);

  assert.doesNotThrow(() =>
    assertSafeOrigin("http://127.0.0.1:9000/internal/tools/v1/search", policy),
  );
  for (const origin of [
    "http://127.0.0.1:6379/",
    "http://localhost:6379/",
    "http://127.0.0.1:9001/",
    "https://127.0.0.1:8443/",
  ]) {
    assert.throws(
      () => assertSafeOrigin(origin, policy),
      /出站目的地被拒绝/,
      origin,
    );
  }

  // A localhost backend must not make other localhost ports reachable.
  const loopbackBackend = resolveAllowedTargets("http://localhost:9000", "");
  assert.doesNotThrow(() =>
    assertSafeOrigin("http://localhost:9000/api/health", loopbackBackend),
  );
  assert.throws(
    () => assertSafeOrigin("https://localhost:443/", loopbackBackend),
    /仅允许精确配置的 origin/,
  );
  assert.throws(
    () => assertSafeOrigin("http://localhost:9001/", loopbackBackend),
    /仅允许精确配置的 origin/,
  );
});

test("resolveAllowedTargets trusts operator-listed hosts for HTTP and private answers", () => {
  const policy = resolveAllowedTargets(
    "http://127.0.0.1:9000",
    "internal.example.com, 192.168.1.5",
  );
  assert.equal(policy.hosts.has("internal.example.com"), true);
  assert.equal(policy.hosts.has("192.168.1.5"), true);
  assert.equal(policy.lookupHosts.has("internal.example.com"), true);
  assert.equal(policy.lookupHosts.has("192.168.1.5"), true);
  assert.doesNotThrow(() =>
    assertSafeOrigin("http://internal.example.com:11434/", policy),
  );
  assert.doesNotThrow(() =>
    assertSafeOrigin("http://192.168.1.5:8080/", policy),
  );
});

test("non-exempt hops must use HTTPS", () => {
  const policy = resolveAllowedTargets("https://backend.example.com", "");
  assert.doesNotThrow(() =>
    assertSafeOrigin("https://backend.example.com/internal/tools/v1/search", policy),
  );
  assert.doesNotThrow(() =>
    assertSafeOrigin("https://api.openai.com/v1/chat/completions", policy),
  );
  assert.doesNotThrow(() => assertSafeOrigin("https://8.8.8.8/", policy));
  assert.throws(
    () => assertSafeOrigin("http://api.openai.com/", policy),
    /仅允许 HTTPS/,
  );
  assert.throws(
    () => assertSafeOrigin("http://8.8.8.8/", policy),
    /仅允许 HTTPS/,
  );
});

test("assertSafeOrigin rejects non-standard IPv4 spellings", () => {
  const policy = resolveAllowedTargets("http://backend.internal:9000", "");
  for (const origin of [
    "https://2130706433/",
    "https://127.1/",
    "https://0x7f000001/",
    "https://0177.0.0.1/",
    "https://127.0.0.1./",
    "https://[::1]/",
    "https://[::ffff:127.0.0.1]/",
  ]) {
    assert.throws(
      () => assertSafeOrigin(origin, policy),
      /出站目的地被拒绝/,
      origin,
    );
  }
});

test("assertSafeOrigin allows the exempt origin and public addresses", () => {
  const policy = resolveAllowedTargets("https://api.openai.com", "");
  assert.doesNotThrow(() => assertSafeOrigin("https://api.openai.com/v1", policy));
  assert.doesNotThrow(() => assertSafeOrigin("https://8.8.8.8/", policy));
  assert.throws(
    () => assertSafeOrigin("ftp://example.com/", policy),
    /不是受支持的协议/,
  );
  assert.throws(() => assertSafeOrigin(undefined, policy), /缺少目标地址/);
});

test("guardedLookup blocks hostnames resolving to loopback unless exempt", async () => {
  const blocked = await new Promise((resolve) => {
    guardedLookup(
      "localhost",
      {},
      (error, address) => resolve({ error, address }),
      new Set(),
    );
  });
  assert.equal(blocked.error?.code, "EACCES");
  assert.match(String(blocked.error?.message), /localhost/);

  const exempt = await new Promise((resolve) => {
    guardedLookup(
      "localhost",
      {},
      (error, address) => resolve({ error, address }),
      new Set(["localhost"]),
    );
  });
  assert.equal(exempt.error, null);
  assert.ok(
    exempt.address === "127.0.0.1" || exempt.address === "::1",
    String(exempt.address),
  );
});

test("guardedLookup honours the all option for exempt hosts", async () => {
  const result = await new Promise((resolve) => {
    guardedLookup(
      "localhost",
      { all: true },
      (error, address) => resolve({ error, address }),
      new Set(["localhost"]),
    );
  });
  assert.equal(result.error, null);
  assert.ok(Array.isArray(result.address));
  assert.ok(result.address.length > 0);
  for (const entry of result.address) {
    assert.equal(typeof entry.address, "string");
    assert.ok(entry.family === 4 || entry.family === 6);
  }
});

test("GuardedAgent rejects unsafe literal origins before dispatch", () => {
  const agent = new GuardedAgent();
  let error;
  const handled = agent.dispatch(
    { origin: "https://2130706433/", path: "/", method: "GET" },
    {
      onError(cause) {
        error = cause;
      },
    },
  );
  assert.equal(handled, false);
  assert.equal(error?.code, "EACCES");
  assert.match(String(error?.message), /127\.0\.0\.1/);
});

function listen(server) {
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, () => resolve(server.address().port));
  });
}

test("redirects to other loopback ports are blocked", async (t) => {
  const previousBackend = process.env.CHAT_BACKEND_URL;
  let gateway;
  let target;
  try {
    target = createServer((_request, response) => {
      response.writeHead(200);
      response.end("secret");
    });
    const targetPort = await listen(target);
    gateway = createServer((request, response) => {
      if (request.url === "/ok") {
        response.writeHead(200);
        response.end("gateway");
        return;
      }
      if (request.url === "/redirect-ip") {
        response.writeHead(302, {
          Location: `http://127.0.0.1:${targetPort}/secret`,
        });
        response.end();
        return;
      }
      if (request.url === "/redirect-localhost") {
        response.writeHead(302, {
          Location: `http://localhost:${targetPort}/secret`,
        });
        response.end();
        return;
      }
      response.writeHead(404);
      response.end();
    });
    const gatewayPort = await listen(gateway);

    // Only this exact origin is exempt, matching the default backend setup.
    process.env.CHAT_BACKEND_URL = `http://127.0.0.1:${gatewayPort}`;
    installEgressGuard();

    const ok = await fetch(`http://127.0.0.1:${gatewayPort}/ok`);
    assert.equal(ok.status, 200);

    for (const path of ["/redirect-ip", "/redirect-localhost"]) {
      const failure = await fetch(
        `http://127.0.0.1:${gatewayPort}${path}`,
      ).then(
        () => null,
        (error) => error,
      );
      assert.ok(failure, `${path} must be rejected`);
      const details = [
        failure.message,
        failure.cause?.message,
        failure.cause?.cause?.message,
      ]
        .filter(Boolean)
        .join(" | ");
      assert.match(details, /出站目的地被拒绝/);
    }

    // The redirect target itself must not be reachable directly either.
    const direct = await fetch(`http://127.0.0.1:${targetPort}/secret`).then(
      () => null,
      (error) => error,
    );
    assert.ok(direct, "direct access to another loopback port must be rejected");
  } catch (error) {
    if (error?.code === "EPERM") {
      t.skip("sandbox forbids listening on loopback");
      return;
    }
    throw error;
  } finally {
    process.env.CHAT_BACKEND_URL = previousBackend;
    gateway?.close();
    target?.close();
  }
});
