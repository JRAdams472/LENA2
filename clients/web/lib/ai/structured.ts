// Local structured generation — runs a server-prepared request through a
// LocalEngine and validates the output client-side against the same
// context the server assembled. Mirrors the server's runPrepared: one
// malformed-output retry, then failure.

import type { PreparedAIRequest } from "../types";
import { extractJsonObject } from "./protocol";
import type { EngineMessage, LocalEngine } from "./types";

export class StructuredError extends Error {}

const RETRY_HINT =
  "That was not valid JSON matching the required schema. Reply ONLY with the JSON object.";

// runStructured drives one prepared request: system prompt + JSON output
// contract, context as the user turn, one retry on unparseable output.
export async function runStructured<C, T>(
  engine: LocalEngine,
  prepared: PreparedAIRequest,
  validate: (parsed: unknown, ctx: C) => T[]
): Promise<T[]> {
  const ctx = JSON.parse(prepared.contextJson) as C;
  const messages: EngineMessage[] = [
    {
      role: "system",
      content:
        `${prepared.prompt}\n\n` +
        `Respond with ONLY a JSON object matching this schema:\n` +
        prepared.outputSchemaJson,
    },
    { role: "user", content: prepared.contextJson },
  ];

  for (let attempt = 0; attempt < 2; attempt++) {
    const raw = await engine.chat(messages);
    const obj = extractJsonObject(raw);
    if (obj !== undefined) {
      const out = validate(obj, ctx);
      if (out !== null) return out;
    }
    messages.push(
      { role: "assistant", content: raw },
      { role: "user", content: RETRY_HINT }
    );
  }
  throw new StructuredError("local model returned malformed suggestions");
}
