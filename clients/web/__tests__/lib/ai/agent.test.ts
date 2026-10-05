import { AgentError, runAgent } from "@/lib/ai/agent";
import type { AgentToolSpec, EngineMessage, LocalEngine } from "@/lib/ai/types";

const TOOLS: AgentToolSpec[] = [
  { name: "get_pantry_inventory", description: "pantry", parametersJson: `{"type":"object"}` },
  { name: "get_expiring_items", description: "expiring", parametersJson: `{"type":"object"}` },
];

// Scripted engine — each call shifts the next canned reply. Messages are
// snapshotted because the agent mutates the shared array between calls.
function scriptedEngine(replies: string[]) {
  const seen: EngineMessage[][] = [];
  const engine: LocalEngine = {
    id: "nano",
    label: "test",
    chat: async (messages) => {
      seen.push([...messages]);
      return replies.shift() ?? `{"answer":"done"}`;
    },
    destroy: () => {},
  };
  return { engine, seen };
}

describe("runAgent", () => {
  const callTool = jest.fn(async (name: string, argsJson: string) => `[{"tool":"${name}","args":${argsJson}}]`);

  beforeEach(() => callTool.mockClear());

  it("answers without tools", async () => {
    const { engine } = scriptedEngine([`{"answer":"Nothing's expiring."}`]);
    const out = await runAgent({
      engine, systemPrompt: "sys", tools: TOOLS, callTool, question: "what's expiring?",
    });
    expect(out.answer).toBe("Nothing's expiring.");
    expect(out.tools).toEqual([]);
    expect(callTool).not.toHaveBeenCalled();
  });

  it("runs a tool call then answers", async () => {
    const { engine, seen } = scriptedEngine([
      `{"toolCalls":[{"name":"get_expiring_items","arguments":{"days":14}}]}`,
      `{"answer":"Your milk expires Friday."}`,
    ]);
    const out = await runAgent({
      engine, systemPrompt: "sys", tools: TOOLS, callTool, question: "expiring?",
    });
    expect(callTool).toHaveBeenCalledWith("get_expiring_items", `{"days":14}`);
    expect(out.tools).toEqual(["get_expiring_items"]);
    expect(out.answer).toBe("Your milk expires Friday.");
    // The tool result was fed back as a user turn.
    const lastUser = seen[1].findLast((m) => m.role === "user");
    expect(lastUser?.content).toContain("Tool results:");
    expect(lastUser?.content).toContain("get_expiring_items");
  });

  it("retries once on malformed protocol output", async () => {
    const { engine, seen } = scriptedEngine([
      `{"toolCalls":[{"name":"hallucinated_tool","arguments":{}}]}`,
      `{"answer":"ok"}`,
    ]);
    const out = await runAgent({
      engine, systemPrompt: "sys", tools: TOOLS, callTool, question: "q",
    });
    expect(callTool).not.toHaveBeenCalled();
    expect(out.answer).toBe("ok");
    const nudge = seen[1].at(-1);
    expect(nudge?.role).toBe("user");
    expect(nudge?.content).toContain("ONLY one JSON object");
  });

  it("reports tool transport failures to the model, not a crash", async () => {
    callTool.mockRejectedValueOnce(new Error("network down"));
    const { engine, seen } = scriptedEngine([
      `{"toolCalls":[{"name":"get_pantry_inventory","arguments":{}}]}`,
      `{"answer":"can't reach the pantry right now"}`,
    ]);
    const out = await runAgent({
      engine, systemPrompt: "sys", tools: TOOLS, callTool, question: "q",
    });
    expect(out.answer).toBe("can't reach the pantry right now");
    expect(seen[1].at(-1)?.content).toContain("network down");
  });

  it("aborts past the round cap", async () => {
    const { engine } = scriptedEngine([]);
    // Engine always asks for another tool call (replies.shift() exhausts
    // to the default — override with a loop of calls).
    engine.chat = async () => `{"toolCalls":[{"name":"get_pantry_inventory","arguments":{}}]}`;
    await expect(
      runAgent({ engine, systemPrompt: "sys", tools: TOOLS, callTool, question: "q", maxRounds: 2 })
    ).rejects.toThrow(AgentError);
    // rounds 0,1,2 → 3 calls
    expect(callTool).toHaveBeenCalledTimes(3);
  });
});
