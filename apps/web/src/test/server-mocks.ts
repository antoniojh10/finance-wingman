import { vi } from "vitest";

import en from "../../messages/en.json";

/** In-memory cookie jar standing in for next/headers cookies(). */
export const cookieJar = new Map<string, { value: string; options?: Record<string, unknown> }>();

/** Headers of the incoming browser request, standing in for next/headers headers(). */
export const incomingHeaders = new Headers({ "user-agent": "Mozilla/5.0 (Test Browser)" });

export class RedirectError extends Error {
  constructor(readonly url: string) {
    super(`NEXT_REDIRECT ${url}`);
  }
}

function lookup(path: string): string {
  const value = path.split(".").reduce<unknown>((node, key) => (node as Record<string, unknown>)?.[key], en);
  return typeof value === "string" ? value : path;
}

export function translator(namespace?: string) {
  return (key: string, values: Record<string, unknown> = {}) =>
    lookup(namespace ? `${namespace}.${key}` : key).replace(/\{(\w+)\}/g, (_, name) => String(values[name] ?? ""));
}

vi.mock("next/headers", () => ({
  cookies: async () => ({
    get: (name: string) => (cookieJar.has(name) ? { name, value: cookieJar.get(name)!.value } : undefined),
    set: (name: string, value: string, options?: Record<string, unknown>) => cookieJar.set(name, { value, options }),
    delete: (name: string) => cookieJar.delete(name),
  }),
  headers: async () => incomingHeaders,
}));

vi.mock("next/navigation", () => ({
  redirect: (url: string) => {
    throw new RedirectError(url);
  },
}));

vi.mock("next/cache", () => ({ revalidatePath: vi.fn() }));

vi.mock("next-intl/server", () => ({
  getTranslations: async (namespace?: string) => translator(namespace),
  getLocale: async () => "en",
}));

type Route = { method: string; path: string; status: number; body?: unknown };

/** Stubs fetch with canned API responses and records the requests made. */
export function mockApi(routes: Route[]) {
  const requests: { method: string; path: string; body: unknown; auth: string | null; userAgent: string | null }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: Request) => {
      const url = new URL(input.url);
      const text = await input.text();
      requests.push({
        method: input.method,
        path: url.pathname + url.search,
        body: text ? JSON.parse(text) : undefined,
        auth: input.headers.get("Authorization"),
        userAgent: input.headers.get("User-Agent"),
      });
      const route = routes.find((r) => r.method === input.method && (url.pathname + url.search).startsWith(r.path));
      if (!route) {
        return new Response(JSON.stringify({ status: 404, detail: "not mocked" }), { status: 404 });
      }
      return new Response(route.status === 204 ? null : JSON.stringify(route.body ?? {}), {
        status: route.status,
        headers: { "Content-Type": "application/json" },
      });
    }),
  );
  return requests;
}
