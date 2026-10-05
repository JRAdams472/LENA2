// The shared JSON tool protocol for small local models. Native tool
// calling is unreliable or absent on 1-3B runtimes, so the agent instead
// requires every model reply to be one JSON object:
//
//   {"toolCalls":[{"name":"get_pantry_inventory","arguments":{...}}]}
//   {"answer":"You have 2 gallons of milk."}
//
// Parsing is deliberately forgiving — models wrap JSON in markdown fences
// or prose — but tool names are checked against the served spec list, so
// hallucinated calls turn into a malformed reply the agent can retry.

import type { AgentToolSpec, ToolCallRequest } from "./types";

export type ParsedReply =
  | { kind: "tools"; calls: ToolCallRequest[] }
  | { kind: "answer"; text: string }
  | { kind: "malformed" };

// RETRY_NUDGE is appended as a user turn when the model replies with
// something that parsed as a protocol object but was unusable.
export const RETRY_NUDGE =
  'Reply with ONLY one JSON object: {"toolCalls":[...]} to call tools, or {"answer":"..."} for the final answer.';

// MAX_CALL_ARGS_BYTES mirrors the server's tool argument cap so a runaway
// model can't force oversized payloads over the wire.
export const MAX_CALL_ARGS_BYTES = 4096;

// buildSystemPrompt appends the protocol contract and the served tool
// catalog to the server-owned system prompt.
export function buildSystemPrompt(basePrompt: string, tools: AgentToolSpec[]): string {
  const catalog = tools
    .map((t) => {
      let params = t.parametersJson;
      try {
        params = JSON.stringify(JSON.parse(t.parametersJson));
      } catch {
        /* serve verbatim */
      }
      return `- ${t.name}: ${t.description}\n  args schema: ${params}`;
    })
    .join("\n");
  return `${basePrompt}

TOOL PROTOCOL: tools are read-only lookups; calling one fetches real household data.
To call tools, reply with EXACTLY one JSON object and nothing else:
{"toolCalls":[{"name":"<tool name>","arguments":{<args matching the schema>}}]}
To give the final answer, reply with EXACTLY one JSON object and nothing else:
{"answer":"<your reply>"}
Never mix prose and JSON. Never invent tool names or results.

Available tools:
${catalog}`;
}

// stripFences removes markdown code fences a model may wrap output in.
// Written without regex so model output can't trigger backtracking.
function stripFences(text: string): string {
  const start = text.indexOf("```");
  if (start === -1) return text.trim();
  const nl = text.indexOf("\n", start);
  if (nl === -1) return text.trim();
  // Only a bare ``` or ```json opening fence is stripped — other
  // languages are left alone, matching the previous regex's contract.
  if (!/^```(json)?$/.test(text.slice(start, nl).trim())) {
    return text.trim();
  }
  const close = text.indexOf("```", nl);
  if (close === -1) return text.trim();
  return text.slice(nl + 1, close).trim();
}

// extractJsonObject finds a JSON object in possibly-noisy model output:
// the whole reply first, then the widest brace span as a fallback.
export function extractJsonObject(raw: string): unknown {
  const text = stripFences(raw);
  for (const candidate of [text, text.slice(text.indexOf("{"), text.lastIndexOf("}") + 1)]) {
    if (!candidate?.startsWith("{")) continue;
    try {
      return JSON.parse(candidate);
    } catch {
      /* try the next candidate */
    }
  }
  return undefined;
}

// parseToolCalls normalizes the toolCalls array: each call needs a known
// name and arguments that fit the wire cap (arguments may arrive as an
// object or a stringified object — both occur in the wild).
function parseToolCalls(v: unknown, known: Set<string>): ToolCallRequest[] | undefined {
  if (!Array.isArray(v) || v.length === 0) return undefined;
  const calls = v.map((item) => toToolCall(item, known));
  // One bad call poisons the batch — the model gets a retry, not a
  // silently truncated tool set.
  return calls.includes(undefined) ? undefined : (calls as ToolCallRequest[]);
}

function toToolCall(item: unknown, known: Set<string>): ToolCallRequest | undefined {
  if (!item || typeof item !== "object") return undefined;
  const { name, arguments: args } = item as { name?: unknown; arguments?: unknown };
  if (typeof name !== "string" || !known.has(name)) return undefined;
  const parsed = parseCallArguments(args);
  if (!parsed || JSON.stringify(parsed).length > MAX_CALL_ARGS_BYTES) {
    return undefined;
  }
  return { name, arguments: parsed };
}

// arguments may arrive as an object or a stringified object — both occur
// in the wild. Absent arguments normalize to an empty object.
function parseCallArguments(args: unknown): Record<string, unknown> | undefined {
  if (typeof args === "string" && args.trim()) {
    try {
      const p = JSON.parse(args);
      if (!p || typeof p !== "object" || Array.isArray(p)) return undefined;
      return p as Record<string, unknown>;
    } catch {
      return undefined;
    }
  }
  if (args == null) return {};
  if (typeof args !== "object" || Array.isArray(args)) return undefined;
  return args as Record<string, unknown>;
}

// parseModelReply classifies raw model output for the agent loop.
export function parseModelReply(raw: string, tools: AgentToolSpec[]): ParsedReply {
  const known = new Set(tools.map((t) => t.name));
  const obj = extractJsonObject(raw);

  if (obj && typeof obj === "object" && !Array.isArray(obj)) {
    const rec = obj as Record<string, unknown>;
    if ("toolCalls" in rec) {
      const calls = parseToolCalls(rec.toolCalls, known);
      if (calls) return { kind: "tools", calls };
      // A toolCalls key that didn't produce valid calls is a protocol
      // violation — retry beats silently answering.
      return { kind: "malformed" };
    }
    if (typeof rec.answer === "string" && rec.answer.trim()) {
      return { kind: "answer", text: rec.answer.trim() };
    }
    return { kind: "malformed" };
  }

  // No JSON object at all: the model answered plainly. Accept it rather
  // than erroring — a small model that forgets the protocol still helped.
  const text = stripFences(raw).trim();
  if (text) return { kind: "answer", text };
  return { kind: "malformed" };
}

// formatToolResults serializes executed calls into the next user turn.
export function formatToolResults(results: { name: string; result: string }[]): string {
  const payload = results.map((r) => {
    let parsed: unknown = r.result;
    try {
      parsed = JSON.parse(r.result);
    } catch {
      /* keep the raw string */
    }
    return { name: r.name, result: parsed };
  });
  return `Tool results:\n${JSON.stringify(payload)}`;
}
