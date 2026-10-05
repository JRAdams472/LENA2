// Local-inference capability detection. Order is the runtime chain from
// the megaplan: Chrome's built-in model (zero model download for us) →
// WebLLM over WebGPU (works in any GPU-capable browser) → none.

export type LocalCapability = "nano" | "webllm" | "none";

// Prompt API shapes seen in the wild: the modern `LanguageModel` global
// and the origin-trial-era `window.ai.languageModel`. Feature-detect both.
export interface NanoSession {
  prompt(input: string): Promise<string>;
  destroy?(): void;
}

export interface NanoHandle {
  availability?(): Promise<string>;
  capabilities?(): Promise<{ available?: string }>;
  create(options?: Record<string, unknown>): Promise<NanoSession>;
}

interface NanoGlobal {
  LanguageModel?: NanoHandle;
  ai?: { languageModel?: NanoHandle };
}

export function nanoHandle(g: NanoGlobal = globalThis as NanoGlobal): NanoHandle | null {
  return g.LanguageModel ?? g.ai?.languageModel ?? null;
}

// hasWebGPU reports whether the WebLLM path can run at all.
export function hasWebGPU(g: { navigator?: { gpu?: unknown } } = globalThis): boolean {
  return g.navigator?.gpu !== undefined && g.navigator.gpu !== null;
}

export function detectLocalCapability(g: NanoGlobal & { navigator?: { gpu?: unknown } } = globalThis): LocalCapability {
  if (nanoHandle(g)) return "nano";
  if (hasWebGPU(g)) return "webllm";
  return "none";
}

export type NanoAvailability = "available" | "downloadable" | "unavailable";

// nanoAvailability normalizes the two API eras: availability() returns
// 'available'|'downloadable'|'downloading'|'unavailable'; the older
// capabilities().available uses 'no'|'after-download'|'readily'.
export async function nanoAvailability(g: NanoGlobal = globalThis as NanoGlobal): Promise<NanoAvailability> {
  const h = nanoHandle(g);
  if (!h) return "unavailable";
  try {
    if (h.availability) {
      const a = await h.availability();
      if (a === "available" || a === "readily") return "available";
      if (a === "unavailable" || a === "no") return "unavailable";
      return "downloadable"; // covers "downloadable" and "downloading"
    }
    if (h.capabilities) {
      const c = await h.capabilities();
      if (c.available === "readily") return "available";
      if (c.available === "no") return "unavailable";
      return "downloadable";
    }
  } catch {
    return "unavailable";
  }
  return "unavailable";
}
