// Module-level registry for the active local engine. The assistant page
// creates engines on opt-in; suggestion features (P4) reuse whatever is
// already loaded. Suggestion code paths never trigger a model download
// implicitly — they only use an engine that's ready now, or the
// zero-download built-in (nano) when the browser offers it.

import { useEffect, useState } from "react";
import { detectLocalCapability, nanoAvailability } from "./capabilities";
import { NanoEngine } from "./engines/nano";
import type { LocalEngine } from "./types";

const MODE_KEY = "lena-ai-mode";

let engine: LocalEngine | null = null;
let probing: Promise<LocalEngine | null> | null = null;

export function getLocalEngine(): LocalEngine | null {
  return engine;
}

export function setLocalEngine(e: LocalEngine | null): void {
  engine = e;
}

function serverOnly(): boolean {
  return (
    typeof window !== "undefined" &&
    window.localStorage.getItem(MODE_KEY) === "server"
  );
}

// engineForSuggestions returns a ready local engine or null — creating a
// zero-download nano session lazily if the browser supports it, but never
// starting a WebLLM download here (that needs the explicit opt-in card on
// the assistant page).
export async function engineForSuggestions(): Promise<LocalEngine | null> {
  if (engine) return engine;
  if (serverOnly()) return null;
  if (probing) return probing;
  probing = (async () => {
    if (detectLocalCapability() === "nano" && (await nanoAvailability()) === "available") {
      const created = await NanoEngine.create();
      if (created) engine = created;
      return created;
    }
    return null;
  })().finally(() => {
    probing = null;
  });
  return probing;
}

// useLocalEngineReady reports whether suggestion features can run locally
// right now (engine loaded, or a zero-download nano session possible).
export function useLocalEngineReady(): boolean {
  const [ready, setReady] = useState(false);
  useEffect(() => {
    let on = true;
    void engineForSuggestions().then((e) => {
      if (on) setReady(e !== null);
    });
    return () => {
      on = false;
    };
  }, []);
  return ready;
}
