import { api, setAuthTokenGetter } from "@/lib/api";

const mockFetch = global.fetch as jest.Mock;

function mockGraphQL(data: object | null, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: {
      get: (name: string) =>
        name === "content-type" ? "application/json" : null,
    },
    json: async () => ({ data }),
  };
}

function requestInit(call: number): RequestInit {
  return mockFetch.mock.calls[call][1] as RequestInit;
}

function idemKey(call: number): string | undefined {
  const headers = requestInit(call).headers as Record<string, string>;
  return headers["Idempotency-Key"];
}

describe("api idempotency", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    setAuthTokenGetter(() => null);
  });

  it("sends an Idempotency-Key on mutations", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ recordSearch: true }));
    await api.recordSearch("recipe", "milk");
    expect(idemKey(0)).toMatch(/^[0-9a-z-]+$/i);
  });

  it("does not send an Idempotency-Key on queries", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ me: { id: "1" } }));
    await api.getMe().catch(() => {});
    expect(idemKey(0)).toBeUndefined();
  });

  it("generates a fresh key per logical operation", async () => {
    mockFetch
      .mockResolvedValueOnce(mockGraphQL({ recordSearch: true }))
      .mockResolvedValueOnce(mockGraphQL({ recordSearch: true }));
    await api.recordSearch("recipe", "a");
    await api.recordSearch("recipe", "a");
    expect(idemKey(0)).toBeTruthy();
    expect(idemKey(1)).toBeTruthy();
    expect(idemKey(0)).not.toBe(idemKey(1));
  });

  it("retries a network failure once with the same key", async () => {
    mockFetch
      .mockRejectedValueOnce(new TypeError("fetch failed"))
      .mockResolvedValueOnce(mockGraphQL({ recordSearch: true }));

    await api.recordSearch("recipe", "milk");

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(idemKey(0)).toBeTruthy();
    expect(idemKey(1)).toBe(idemKey(0));
  });

  it("retries queries too — the key only distinguishes mutations", async () => {
    mockFetch
      .mockRejectedValueOnce(new TypeError("fetch failed"))
      .mockResolvedValueOnce(mockGraphQL({ me: { id: "1" } }));

    await api.getMe().catch(() => {});

    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it("propagates a second consecutive network failure", async () => {
    mockFetch
      .mockRejectedValueOnce(new TypeError("fetch failed"))
      .mockRejectedValueOnce(new TypeError("fetch failed"));

    await expect(api.recordSearch("recipe", "milk")).rejects.toThrow(
      "fetch failed"
    );
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it("does not retry HTTP errors", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL(null, 500));

    await expect(api.recordSearch("recipe", "milk")).rejects.toThrow();
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});
