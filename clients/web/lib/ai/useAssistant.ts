// Assistant orchestration for the web client: resolves which engine
// answers a question — an on-device runtime when one is set up, else the
// server — and owns the opt-in/download lifecycle for local models.

"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { runAgent } from "./agent";
import { detectLocalCapability, nanoAvailability, type LocalCapability } from "./capabilities";
import { NanoEngine } from "./engines/nano";
import { DEFAULT_WEBLLM_MODEL, WebLLMEngine, WEBLLM_MODELS, type DownloadProgress, type WebLLMModelKey } from "./engines/webllm";
import type { AssistantResult, LocalEngine } from "./types";

export type AssistantMode = "auto" | "server";

const MODE_KEY = "lena-ai-mode";
const MODEL_KEY = "lena-ai-model";

function loadMode(): AssistantMode {
  if (typeof window === "undefined") return "auto";
  return window.localStorage.getItem(MODE_KEY) === "server" ? "server" : "auto";
}

function loadModel(): WebLLMModelKey {
  if (typeof window === "undefined") return DEFAULT_WEBLLM_MODEL;
  const k = window.localStorage.getItem(MODEL_KEY);
  return k && k in WEBLLM_MODELS ? (k as WebLLMModelKey) : DEFAULT_WEBLLM_MODEL;
}

export type LocalStatus =
  | "unavailable" // no local runtime on this browser
  | "ready" // an engine is initialized and will be used
  | "opt-in" // local possible but not enabled (download/consent needed)
  | "downloading"; // opt-in granted, model download in flight

export interface AssistantController {
  // Whether Dot can answer at all (server provider or local engine).
  available: boolean;
  // null while capability probing is still running.
  status: LocalStatus | null;
  // Label of the engine that produced the last answer, or will produce
  // the next one — drives the "On this device" / "Via server" badge.
  engineLabel: string;
  // Whether the next ask will use a local engine.
  localActive: boolean;
  mode: AssistantMode;
  setServerOnly(v: boolean): void;
  // Begin (or resume) local setup: nano session or a model download.
  enableLocal(): Promise<void>;
  // True while a model download is in flight; progress is 0..1.
  download: DownloadProgress | null;
  downloadError: string | null;
  ask(question: string): Promise<AssistantResult>;
}

export function useAssistant(): AssistantController {
  const qc = useQueryClient();
  const { data: serverAI } = useQuery({ queryKey: ["aiAvailable"], queryFn: api.getAIAvailable });
  const { data: tools } = useQuery({ queryKey: ["assistantTools"], queryFn: api.getAssistantTools });

  const [capability, setCapability] = useState<LocalCapability | null>(null);
  const [nanoReady, setNanoReady] = useState<boolean | null>(null);
  const [mode, setMode] = useState<AssistantMode>("auto");
  const [engine, setEngine] = useState<LocalEngine | null>(null);
  const [download, setDownload] = useState<DownloadProgress | null>(null);
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const engineRef = useRef<LocalEngine | null>(null);
  engineRef.current = engine;

  // Probe capabilities once on mount.
  useEffect(() => {
    setMode(loadMode());
    const cap = detectLocalCapability();
    setCapability(cap);
    if (cap === "nano") {
      nanoAvailability().then((a) => setNanoReady(a === "available"));
    } else {
      setNanoReady(null);
    }
  }, []);

  const persistMode = (m: AssistantMode) => {
    setMode(m);
    window.localStorage.setItem(MODE_KEY, m);
  };

  const setServerOnly = useCallback((serverOnly: boolean) => {
    persistMode(serverOnly ? "server" : "auto");
  }, []);

  const enableLocal = useCallback(async () => {
    setDownloadError(null);
    persistMode("auto");
    const cap = capability ?? detectLocalCapability();
    try {
      if (cap === "nano") {
        const e = await NanoEngine.create();
        if (!e) throw new Error("the built-in model isn't ready on this browser");
        setEngine(e);
        return;
      }
      if (cap === "webllm") {
        const key = loadModel();
        setDownload({ fraction: 0, text: "Preparing…" });
        const e = await WebLLMEngine.create(key, setDownload);
        setDownload(null);
        setEngine(e);
        return;
      }
      throw new Error("this browser can't run a local model");
    } catch (err) {
      setDownload(null);
      setDownloadError(err instanceof Error ? err.message : "Local setup failed");
    }
  }, [capability]);

  const ask = useCallback(
    async (question: string): Promise<AssistantResult> => {
      const currentMode = loadMode();
      let local = engineRef.current;

      // Auto mode: a zero-download nano session is created on first use.
      if (currentMode !== "server" && !local && capability === "nano" && nanoReady) {
        local = await NanoEngine.create();
        if (local) setEngine(local);
      }

      if (currentMode !== "server" && local && tools && tools.length > 0) {
        try {
          const systemPrompt = await qc.fetchQuery({
            queryKey: ["assistantPrompt", "ask"],
            queryFn: () => api.getAssistantPrompt("ask"),
            staleTime: Infinity,
          });
          const out = await runAgent({
            engine: local,
            systemPrompt,
            tools,
            callTool: api.callAssistantTool,
            question,
          });
          return { ...out, engine: local.id };
        } catch {
          // Local inference failed (model unloaded, OOM, protocol drift) —
          // transparently fall back to the server for this turn and drop
          // the broken engine so the next ask doesn't retry it.
          local.destroy();
          setEngine((prev) => (prev === local ? null : prev));
          if (!serverAI) throw new Error("the local model didn't answer and no server provider is configured");
        }
      }

      const data = await api.askAssistant(question);
      return {
        answer: data.answer,
        tools: data.toolCalls.map((t) => t.name),
        engine: "server",
      };
    },
    [capability, nanoReady, tools, serverAI, qc]
  );

  const toolsReady = !!tools && tools.length > 0;
  const localPossible = toolsReady && (capability === "nano" ? nanoReady !== false : capability === "webllm");

  const status: LocalStatus | null = capability === null
    ? null
    : download
      ? "downloading"
      : engine
        ? "ready"
        : localPossible
          ? "opt-in"
          : "unavailable";

  const available = serverAI === true || status === "ready" || (localPossible && status !== "unavailable");
  const localActive = mode !== "server" && engine !== null;

  return {
    available,
    status,
    engineLabel: localActive ? engine.label : "Via server",
    localActive,
    mode,
    setServerOnly,
    enableLocal,
    download,
    downloadError,
    ask,
  };
}
