/**
 * @jest-environment node
 */
import { NextRequest } from "next/server";
import { proxy } from "../proxy";

const makeReq = () => new NextRequest("http://localhost/login");

const responseCsp = (res: ReturnType<typeof proxy>) =>
  res.headers.get("content-security-policy") ?? "";

describe("proxy CSP", () => {
  it("issues a fresh nonce per request", () => {
    const a = responseCsp(proxy(makeReq()));
    const b = responseCsp(proxy(makeReq()));
    const nonce = (h: string) => h.match(/'nonce-([^']+)'/)?.[1];
    expect(nonce(a)).toBeTruthy();
    expect(nonce(a)).not.toBe(nonce(b));
  });

  it("drops 'unsafe-inline' from script-src and keeps strict-dynamic", () => {
    const scriptSrc = responseCsp(proxy(makeReq()))
      .split(";")
      .map((d) => d.trim())
      .find((d) => d.startsWith("script-src"));
    expect(scriptSrc).toContain("'self'");
    expect(scriptSrc).toContain("'strict-dynamic'");
    expect(scriptSrc).not.toContain("'unsafe-inline'");
  });

  it("keeps 'unsafe-inline' on style-src for Emotion/MUI", () => {
    const styleSrc = responseCsp(proxy(makeReq()))
      .split(";")
      .map((d) => d.trim())
      .find((d) => d.startsWith("style-src"));
    expect(styleSrc).toContain("'unsafe-inline'");
  });

  it("carries the rest of the Caddy policy verbatim", () => {
    const csp = responseCsp(proxy(makeReq()));
    for (const directive of [
      "default-src 'self'",
      "img-src 'self' data: blob:",
      "font-src 'self' data:",
      "connect-src 'self' https://accounts.google.com/gsi/",
      "frame-src https://accounts.google.com/gsi/",
      "object-src 'none'",
      "base-uri 'none'",
      "form-action 'self'",
      "frame-ancestors 'none'",
    ]) {
      expect(csp).toContain(directive);
    }
  });

  it("passes the nonce and CSP through request headers for Next to consume", () => {
    const res = proxy(makeReq());
    const reqNonce = res.headers.get("x-middleware-request-x-nonce");
    const reqCsp = res.headers.get(
      "x-middleware-request-content-security-policy"
    );
    const resNonce = responseCsp(res).match(/'nonce-([^']+)'/)?.[1];
    expect(reqNonce).toBe(resNonce);
    expect(reqCsp).toBe(responseCsp(res));
  });
});
