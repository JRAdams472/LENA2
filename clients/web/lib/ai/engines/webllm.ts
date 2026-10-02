// WebLLM (MLC) engine — the WebGPU path. The package and the model
// weights are both lazy: the module is dynamically imported and the
// model downloads on first use, so neither hits the initial bundle.

import type { EngineMessage, LocalEngine } from "../types";

// Model allowlist. Keep to MLC prebuilt ids — the ids double as the
// Hugging Face artifact names, so only well-known quantized SLMs land here.
export const WEBLLM_MODELS = {
  // ~750 MB — the default; fast enough on integrated GPUs.
  "llama-3.2-1b": {
    modelId: "Llama-3.2-1B-Instruct-q4f16_1-MLC",
    sizeLabel: "~750 MB",
    label: "Llama 3.2 1B",
  },
  // ~1.6 GB — noticeably better instruction-following on discrete GPUs.
  "llama-3.2-3b": {
    modelId: "Llama-3.2-3B-Instruct-q4f16_1-MLC",
    sizeLabel: "~1.6 GB",
    label: "Llama 3.2 3B",
  },
} as const;

export type WebLLMModelKey = keyof typeof WEBLLM_MODELS;
export const DEFAULT_WEBLLM_MODEL: WebLLMModelKey = "llama-3.2-1b";

export interface DownloadProgress {
  // 0..1 fraction across the whole model download+init.
  fraction: number;
  // WebLLM's human-readable status line ("Fetching model weights…").
  text: string;
}

export type ProgressCallback = (p: DownloadProgress) => void;

interface MlcInitReport {
  progress: number;
  text: string;
}

interface MlcChatCompletion {
  choices: { message?: { content?: string | null } }[];
}

interface MlcEngine {
  chat: {
    completions: {
      create(req: {
        messages: { role: string; content: string }[];
        temperature?: number;
        max_tokens?: number;
      }): Promise<MlcChatCompletion>;
    };
  };
  unload?(): Promise<void>;
}

export class WebLLMEngine implements LocalEngine {
  readonly id = "webllm" as const;
  readonly label: string;

  private constructor(
    private readonly engine: MlcEngine,
    model: { label: string }
  ) {
    this.label = `On this device (${model.label})`;
  }

  // create downloads and initializes the model — potentially minutes on
  // first run; onProgress streams fraction updates for the UI.
  static async create(model: WebLLMModelKey, onProgress?: ProgressCallback): Promise<WebLLMEngine> {
    const webllm = await import("@mlc-ai/web-llm");
    const spec = WEBLLM_MODELS[model];
    const engine = (await webllm.CreateMLCEngine(spec.modelId, {
      initProgressCallback: (r: MlcInitReport) =>
        onProgress?.({ fraction: r.progress, text: r.text }),
    })) as unknown as MlcEngine;
    return new WebLLMEngine(engine, spec);
  }

  async chat(messages: EngineMessage[]): Promise<string> {
    const res = await this.engine.chat.completions.create({
      messages: messages.map((m) => ({ role: m.role, content: m.content })),
      temperature: 0.3,
      max_tokens: 1024,
    });
    return res.choices[0]?.message?.content ?? "";
  }

  destroy(): void {
    void this.engine.unload?.();
  }
}
