// Shared types for the local-inference assistant path. A LocalEngine is
// any on-device chat runtime (Chrome built-in model, WebLLM, ...); the
// agent loop in agent.ts drives engines through a shared JSON tool
// protocol because 1-3B models lack reliable native function-calling.

// One assistant tool spec as served by the assistantTools query.
export interface AgentToolSpec {
  name: string;
  description: string;
  parametersJson: string;
}

export interface EngineMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

// One tool invocation the model asked for.
export interface ToolCallRequest {
  name: string;
  arguments: Record<string, unknown>;
}

export interface LocalEngine {
  // Stable identifier for badges/telemetry.
  readonly id: "nano" | "webllm";
  // Human label, e.g. "On this device (Gemma)".
  readonly label: string;
  // One completion over the whole conversation so far. Returns the raw
  // model text — parsing is the protocol layer's job.
  chat(messages: EngineMessage[]): Promise<string>;
  // Release model resources (context, VRAM) when the engine is dropped.
  destroy(): void;
}

export type AssistantEngineId = "nano" | "webllm" | "server";

// The outcome of one assistant turn — same shape regardless of where
// inference ran.
export interface AssistantResult {
  answer: string;
  tools: string[];
  engine: AssistantEngineId;
}
