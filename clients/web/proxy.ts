import { NextRequest, NextResponse } from "next/server";

// Nonce-based Content Security Policy (LEN-29 finding 7). The Caddyfile
// no longer owns CSP for page responses — a per-request nonce lets us
// drop 'unsafe-inline' from script-src while Next stamps the nonce on
// every framework bundle automatically. Google Identity Services loads
// via @react-oauth/google's dynamic <script> insertion, which
// 'strict-dynamic' trusts because the inserting script is nonced.
// style-src keeps 'unsafe-inline': Emotion (MUI) injects style tags and
// inline styles are a far weaker injection surface than script.
export function proxy(request: NextRequest) {
  const nonce = Buffer.from(crypto.randomUUID()).toString("base64");
  const isDev = process.env.NODE_ENV === "development";
  const cspHeader = `
    default-src 'self';
    script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${isDev ? " 'unsafe-eval'" : ""};
    style-src 'self' 'unsafe-inline' https://accounts.google.com/gsi/;
    img-src 'self' data: blob:;
    font-src 'self' data:;
    connect-src 'self' https://accounts.google.com/gsi/;
    frame-src https://accounts.google.com/gsi/;
    object-src 'none';
    base-uri 'none';
    form-action 'self';
    frame-ancestors 'none'
  `;
  const contentSecurityPolicyHeaderValue = cspHeader
    .replace(/\s{2,}/g, " ")
    .trim();

  // Next parses the nonce out of the *request* CSP header to stamp it on
  // framework scripts; x-nonce exposes it to Server Components that need
  // to nonce a <Script> manually.
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set(
    "Content-Security-Policy",
    contentSecurityPolicyHeaderValue
  );

  const response = NextResponse.next({
    request: { headers: requestHeaders },
  });
  response.headers.set(
    "Content-Security-Policy",
    contentSecurityPolicyHeaderValue
  );
  return response;
}

export const config = {
  matcher: [
    // Everything except static assets and prefetches — the CSP belongs
    // on HTML documents, not chunks or data requests.
    {
      source: "/((?!_next/static|_next/image|favicon.ico|icon.svg).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
