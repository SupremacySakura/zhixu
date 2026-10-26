import { registerHooks } from 'node:module';
import { test } from 'node:test';
import assert from 'node:assert/strict';

registerHooks({
  resolve(specifier, context, next) {
    if (specifier === '@mastra/core/server') return { url: 'healthmock:server', shortCircuit: true };
    try { return next(specifier, context); }
    catch (error) {
      if (specifier.startsWith('.') && !specifier.endsWith('.ts')) return next(specifier + '.ts', context);
      throw error;
    }
  },
  load(url, context, next) {
    if (url === 'healthmock:server') return {
      format: 'module', shortCircuit: true,
      source: 'export const registerApiRoute = (path, options) => ({path,...options});',
    };
    return next(url, context);
  },
});
const { healthRoute } = await import('../mastra/health.ts');
function context(token) {
  const headers = new Headers();
  return {
    req: { header: name => name === 'authorization' ? token : undefined },
    header: (name, value) => headers.set(name, value),
    json: (data, status = 200) => Response.json(data, { status, headers }),
  };
}
test('health GET returns only liveness with internal auth and no cache', async () => {
  const previous = process.env.INTERNAL_API_SECRET;
  try {
    process.env.INTERNAL_API_SECRET = 'test-secret';
    assert.equal(healthRoute.path, '/internal/health');
    assert.equal(healthRoute.method, 'GET');
    const response = await healthRoute.handler(context('Bearer test-secret'));
    assert.equal(response.status, 200);
    assert.equal(response.headers.get('cache-control'), 'no-store');
    assert.deepEqual(await response.json(), { status: 'ok' });
  } finally {
    if (previous === undefined) delete process.env.INTERNAL_API_SECRET;
    else process.env.INTERNAL_API_SECRET = previous;
  }
});
test('health rejects wrong, missing and unconfigured internal secrets', async () => {
  const previous = process.env.INTERNAL_API_SECRET;
  try {
    for (const secret of ['test-secret', '', '   ']) {
      process.env.INTERNAL_API_SECRET = secret;
      for (const token of [undefined, 'Bearer wrong']) {
        const response = await healthRoute.handler(context(token));
        assert.equal(response.status, 403);
        assert.deepEqual(await response.json(), { error: 'Forbidden' });
      }
    }
  } finally {
    if (previous === undefined) delete process.env.INTERNAL_API_SECRET;
    else process.env.INTERNAL_API_SECRET = previous;
  }
});
