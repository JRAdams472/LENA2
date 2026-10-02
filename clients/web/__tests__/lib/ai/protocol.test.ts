import { buildSystemPrompt, formatToolResults, parseModelReply } from "@/lib/ai/protocol";
import type { AgentToolSpec } from "@/lib/ai/types";

const TOOLS: AgentToolSpec[] = [
  {
    name: "get_pantry_inventory",
    description: "List pantry items",
    parametersJson: `{"type":"object","properties":{"limit":{"type":"integer"}}}`,
  },
];

describe("buildSystemPrompt", () => {
  it("appends the protocol contract and tool catalog", () => {
    const out = buildSystemPrompt("You are Dot.", TOOLS);
    expect(out).toContain("You are Dot.");
    expect(out).toContain("get_pantry_inventory");
    expect(out).toContain('"toolCalls"');
    expect(out).toContain('"answer"');
  });
});

describe("parseModelReply", () => {
  it("parses a tool call", () => {
    const raw = `{"toolCalls":[{"name":"get_pantry_inventory","arguments":{"limit":5}}]}`;
    const r = parseModelReply(raw, TOOLS);
    expect(r).toEqual({
      kind: "tools",
      calls: [{ name: "get_pantry_inventory", arguments: { limit: 5 } }],
    });
  });

  it("parses a final answer", () => {
    const r = parseModelReply(`{"answer":"You have milk."}`, TOOLS);
    expect(r).toEqual({ kind: "answer", text: "You have milk." });
  });

  it("unwraps markdown fences", () => {
    const r = parseModelReply("```json\n{\"answer\":\"hi\"}\n```", TOOLS);
    expect(r).toEqual({ kind: "answer", text: "hi" });
  });

  it("extracts JSON surrounded by prose", () => {
    const r = parseModelReply(`Sure! {"answer":"2 cups"} hope that helps`, TOOLS);
    expect(r).toEqual({ kind: "answer", text: "2 cups" });
  });

  it("accepts stringified arguments", () => {
    const raw = `{"toolCalls":[{"name":"get_pantry_inventory","arguments":"{\\"limit\\":3}"}]}`;
    const r = parseModelReply(raw, TOOLS);
    expect(r).toEqual({
      kind: "tools",
      calls: [{ name: "get_pantry_inventory", arguments: { limit: 3 } }],
    });
  });

  it("rejects hallucinated tool names", () => {
    const raw = `{"toolCalls":[{"name":"drop_table","arguments":{}}]}`;
    expect(parseModelReply(raw, TOOLS)).toEqual({ kind: "malformed" });
  });

  it("rejects oversized arguments", () => {
    const big = "x".repeat(5000);
    const raw = `{"toolCalls":[{"name":"get_pantry_inventory","arguments":{"blob":"${big}"}}]}`;
    expect(parseModelReply(raw, TOOLS)).toEqual({ kind: "malformed" });
  });

  it("treats plain prose as an answer (graceful fallback)", () => {
    const r = parseModelReply("Just answer: you have 2 eggs.", TOOLS);
    expect(r).toEqual({ kind: "answer", text: "Just answer: you have 2 eggs." });
  });

  it("flags empty output as malformed", () => {
    expect(parseModelReply("", TOOLS)).toEqual({ kind: "malformed" });
    expect(parseModelReply("{}", TOOLS)).toEqual({ kind: "malformed" });
  });
});

describe("formatToolResults", () => {
  it("serializes results into a user turn", () => {
    const out = formatToolResults([
      { name: "get_expiring_items", result: `[{"name":"milk"}]` },
      { name: "bad_tool", result: `{"error":"unknown tool"}` },
    ]);
    expect(out).toContain("Tool results:");
    expect(out).toContain("milk");
    const parsed = JSON.parse(out.replace("Tool results:\n", ""));
    expect(parsed[0].result[0].name).toBe("milk");
    expect(parsed[1].result.error).toContain("unknown tool");
  });
});
