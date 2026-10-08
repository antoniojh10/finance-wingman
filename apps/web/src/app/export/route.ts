import { type NextRequest } from "next/server";

import { apiBaseUrl } from "@/lib/api/client";
import { getSessionToken } from "@/lib/session";

const FORMATS = ["json", "csv"];

// Proxies the workspace export so the browser never calls the API directly.
// The body is streamed through, never buffered.
export async function GET(request: NextRequest) {
  const token = await getSessionToken();
  if (!token) {
    return new Response("Unauthorized", { status: 401 });
  }
  const format = request.nextUrl.searchParams.get("format") ?? "json";
  if (!FORMATS.includes(format)) {
    return new Response("Unsupported format", { status: 400 });
  }

  const upstream = await fetch(`${apiBaseUrl()}/api/v1/export?format=${format}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  }).catch(() => null);
  if (!upstream) {
    return new Response("The export is unavailable", { status: 502 });
  }
  if (!upstream.ok) {
    // An expired session or a missing workspace is the user's to fix.
    const status = upstream.status === 401 || upstream.status === 409 ? upstream.status : 502;
    return new Response("The export failed", { status });
  }

  const headers = new Headers({ "Cache-Control": "no-store" });
  for (const name of ["Content-Type", "Content-Disposition"]) {
    const value = upstream.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new Response(upstream.body, { headers });
}
