// The client-side agent loop — mirrors the server's Ask: model emits
// JSON tool calls, each executes server-side through callAssistantTool
// (household-scoped, read-only), results feed back, and the loop ends on
// a final answer or the round cap.

import { buildSystemPrompt, formatToolResults, parseModelReply, RETRY_NUDGE } from "./protocol";
import type { AgentToolSpec, EngineMessage, LocalEngine } from "./types";

export interface AgentRunOptions {
  engine: LocalEngine;
  // Server-owned ask prompt — the same text the server path uses.
  systemPrompt: string;
  tools: AgentToolSpec[];
  callTool: (name: string, argsJson: string) => Promise<string>;
  question: string;
  // Tool-call round trips allowed before giving up. Default 5 — same as
  // the server loop's MaxToolRounds.
  maxRounds?: number;
}

export class AgentError extends Error {}

export async function runAgent(opts: AgentRunOptions): Promise<{ answer: string; tools: string[] }> {
  const { engine, systemPrompt, tools, callTool, question } = opts;
  const maxRounds = opts.maxRounds ?? 5;

  const messages: EngineMessage[] = [
    { role: "system", content: buildSystemPrompt(systemPrompt, tools) },
    { role: "user", content: question },
  ];
  const usedTools: string[] = [];

  for (let round = 0; round <= maxRounds; round++) {
    const raw = await engine.chat(messages);
    messages.push({ role: "assistant", content: raw });
    const parsed = parseModelReply(raw, tools);

    if (parsed.kind === "answer") {
      return { answer: parsed.text, tools: usedTools };
    }
    if (parsed.kind === "malformed") {
      messages.push({ role: "user", content: RETRY_NUDGE });
      continue;
    }

    const results: { name: string; result: string }[] = [];
    for (const call of parsed.calls) {
      usedTools.push(call.name);
      let result: string;
      try {
        result = await callTool(call.name, JSON.stringify(call.arguments));
      } catch (err) {
        // Network/server failures are reported to the model the same way
        // the server loop reports handler errors — as data, not a crash.
        result = JSON.stringify({ error: err instanceof Error ? err.message : String(err) });
      }
      results.push({ name: call.name, result });
    }
    messages.push({ role: "user", content: formatToolResults(results) });
  }
  throw new AgentError(`assistant exceeded ${maxRounds} tool rounds`);
}
