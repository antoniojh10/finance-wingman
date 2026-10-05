const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const datePattern = /^\d{4}-\d{2}-\d{2}$/;

export type TransactionQuery = {
  account_id?: string;
  category_id?: string;
  type?: "expense" | "income" | "transfer";
  from?: string;
  to?: string;
  q?: string;
  page: number;
};

/** Sanitizes transaction list search params, dropping invalid values. */
export function parseTransactionQuery(params: Record<string, string | string[] | undefined>): TransactionQuery {
  const get = (key: string) => {
    const value = params[key];
    return (Array.isArray(value) ? value[0] : value)?.trim() || undefined;
  };
  const type = get("type");
  const page = Number.parseInt(get("page") ?? "1", 10);
  return {
    account_id: uuidPattern.test(get("account_id") ?? "") ? get("account_id") : undefined,
    category_id: uuidPattern.test(get("category_id") ?? "") ? get("category_id") : undefined,
    type: type === "expense" || type === "income" || type === "transfer" ? type : undefined,
    from: datePattern.test(get("from") ?? "") ? get("from") : undefined,
    to: datePattern.test(get("to") ?? "") ? get("to") : undefined,
    q: get("q")?.slice(0, 100),
    page: Number.isFinite(page) && page > 0 ? page : 1,
  };
}

/** Builds a URL query string for a page, keeping active filters. */
export function pageHref(query: TransactionQuery, page: number): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && key !== "page") {
      params.set(key, String(value));
    }
  }
  if (page > 1) {
    params.set("page", String(page));
  }
  const qs = params.toString();
  return qs ? `?${qs}` : "?";
}

/**
 * Returns a same-origin path to go back to after a flow, or the fallback.
 * Rejects absolute and protocol-relative URLs to avoid open redirects.
 */
export function safeReturnPath(value: string | string[] | undefined, fallback = "/"): string {
  const path = Array.isArray(value) ? value[0] : value;
  if (!path || !path.startsWith("/") || path.startsWith("//") || path.startsWith("/\\")) {
    return fallback;
  }
  return path;
}

/** Link to the new transaction page that returns to `returnTo` afterwards. */
export function newTransactionHref(returnTo: string): string {
  return returnTo === "/" ? "/transactions/new" : `/transactions/new?return=${encodeURIComponent(returnTo)}`;
}

/** Builds a filter URL with some values changed, always back on the first page. */
export function filterHref(query: TransactionQuery, changes: Partial<Omit<TransactionQuery, "page">>): string {
  return pageHref({ ...query, ...changes, page: 1 }, 1);
}
