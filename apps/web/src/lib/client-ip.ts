import "server-only";

import { headers } from "next/headers";

/**
 * Picks the client address from an X-Forwarded-For value. `hops` is the number
 * of trusted reverse proxies in front of this server; each appends the address
 * it saw, so the entry `hops` from the right is the one a client cannot forge.
 */
export function clientIpFrom(forwardedFor: string | null, hops: number): string | undefined {
  if (!forwardedFor || hops < 1) {
    return undefined;
  }
  const entries = forwardedFor.split(",").map((entry) => entry.trim());
  return entries.length >= hops ? entries[entries.length - hops] || undefined : undefined;
}

/**
 * Headers that tell the API which browser a request came from. The web server
 * reaches the API over the private network, so without them every visitor
 * would share the web server's address in the API's per-IP rate limits. The
 * API only trusts them together with the shared CLIENT_IP_SECRET.
 */
export async function clientIpHeaders(): Promise<Record<string, string>> {
  const secret = process.env.CLIENT_IP_SECRET;
  if (!secret) {
    return {};
  }
  const hops = Number(process.env.TRUSTED_PROXY_HOPS ?? (process.env.NODE_ENV === "production" ? 1 : 0));
  const ip = clientIpFrom((await headers()).get("x-forwarded-for"), hops);
  return ip ? { "X-Client-IP": ip, "X-Client-IP-Secret": secret } : {};
}
