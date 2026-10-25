// The internal bearer proves the caller is Go, which owns end-user authorization.
export function authorized(c: {
  req: { header: (name: string) => string | undefined };
}): boolean {
  const secret = process.env.INTERNAL_API_SECRET;
  if (!secret?.trim()) return false;
  return c.req.header("authorization") === `Bearer ${secret}`;
}
