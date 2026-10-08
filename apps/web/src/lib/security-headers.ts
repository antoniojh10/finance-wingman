export type SecurityHeadersOptions = {
  nonce: string;
  /** `next dev` needs eval (React debugging) and the HMR websocket. */
  isDev: boolean;
};

/** The Content-Security-Policy for one response, bound to its script nonce. */
export function contentSecurityPolicy({ nonce, isDev }: SecurityHeadersOptions): string {
  const directives = [
    "default-src 'self'",
    // 'strict-dynamic' lets the nonced framework scripts load their chunks.
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${isDev ? " 'unsafe-eval'" : ""}`,
    // Styles stay 'unsafe-inline' (no nonce, which would make it ignored):
    // sonner injects an un-nonced <style> at import time, and components
    // render style="" attributes (positioning, chart colors). Scripts are
    // what the nonce protects.
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' blob: data:",
    "font-src 'self'",
    isDev ? "connect-src 'self' ws: wss:" : "connect-src 'self'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ];
  return directives.join("; ");
}

/** Every header (including the CSP) sent with each HTML response. */
export function securityHeaders(options: SecurityHeadersOptions): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Security-Policy": contentSecurityPolicy(options),
    "X-Content-Type-Options": "nosniff",
    "Referrer-Policy": "strict-origin-when-cross-origin",
    "Permissions-Policy": "camera=(), microphone=(), geolocation=()",
  };
  if (!options.isDev) {
    headers["Strict-Transport-Security"] = "max-age=63072000; includeSubDomains";
  }
  return headers;
}
