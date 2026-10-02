import {
  detectLocalCapability,
  nanoAvailability,
  type NanoHandle,
} from "@/lib/ai/capabilities";

describe("detectLocalCapability", () => {
  it("prefers the built-in Prompt API when present", () => {
    const g = { LanguageModel: {} as NanoHandle, navigator: { gpu: {} } };
    expect(detectLocalCapability(g)).toBe("nano");
  });

  it("accepts the origin-trial window.ai shape", () => {
    const g = { ai: { languageModel: {} as NanoHandle } };
    expect(detectLocalCapability(g)).toBe("nano");
  });

  it("falls back to WebLLM when only WebGPU exists", () => {
    const g = { navigator: { gpu: {} } };
    expect(detectLocalCapability(g)).toBe("webllm");
  });

  it("reports none on a plain browser", () => {
    expect(detectLocalCapability({ navigator: {} })).toBe("none");
  });
});

describe("nanoAvailability", () => {
  const mk = (impl: Partial<NanoHandle>): NanoHandle => impl as NanoHandle;

  it("maps modern availability() values", async () => {
    await expect(
      nanoAvailability({ LanguageModel: mk({ availability: async () => "available" }) })
    ).resolves.toBe("available");
    await expect(
      nanoAvailability({ LanguageModel: mk({ availability: async () => "downloadable" }) })
    ).resolves.toBe("downloadable");
    await expect(
      nanoAvailability({ LanguageModel: mk({ availability: async () => "unavailable" }) })
    ).resolves.toBe("unavailable");
  });

  it("maps the legacy capabilities() shape", async () => {
    await expect(
      nanoAvailability({
        ai: { languageModel: mk({ capabilities: async () => ({ available: "readily" }) }) },
      })
    ).resolves.toBe("available");
    await expect(
      nanoAvailability({
        ai: { languageModel: mk({ capabilities: async () => ({ available: "no" }) }) },
      })
    ).resolves.toBe("unavailable");
  });

  it("is unavailable when probing throws or no handle exists", async () => {
    await expect(
      nanoAvailability({ LanguageModel: mk({ availability: async () => { throw new Error("x"); } }) })
    ).resolves.toBe("unavailable");
    await expect(nanoAvailability({})).resolves.toBe("unavailable");
  });
});
