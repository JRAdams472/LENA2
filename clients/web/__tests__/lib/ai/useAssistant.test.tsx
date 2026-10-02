import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useAssistant } from "../../../lib/ai/useAssistant";
import { api } from "../../../lib/api";
import { detectLocalCapability, nanoAvailability } from "../../../lib/ai/capabilities";
import { NanoEngine } from "../../../lib/ai/engines/nano";
import { WebLLMEngine } from "../../../lib/ai/engines/webllm";
import type { LocalEngine } from "../../../lib/ai/types";

jest.mock("../../../lib/api", () => ({
  api: {
    getAIAvailable: jest.fn(),
    getAssistantTools: jest.fn(),
    getAssistantPrompt: jest.fn(),
    callAssistantTool: jest.fn(),
    askAssistant: jest.fn(),
  },
}));

jest.mock("../../../lib/ai/capabilities", () => ({
  detectLocalCapability: jest.fn(),
  nanoAvailability: jest.fn(),
}));

jest.mock("../../../lib/ai/engines/nano", () => ({
  NanoEngine: { create: jest.fn() },
}));

jest.mock("../../../lib/ai/engines/webllm", () => ({
  DEFAULT_WEBLLM_MODEL: "llama-3.2-1b",
  WEBLLM_MODELS: {
    "llama-3.2-1b": { modelId: "Llama-3.2-1B", sizeLabel: "~750 MB", label: "Llama 3.2 1B" },
  },
  WebLLMEngine: { create: jest.fn() },
}));

const mockedApi = api as jest.Mocked<typeof api>;
const mockCap = detectLocalCapability as jest.Mock;
const mockNanoAvail = nanoAvailability as jest.Mock;
const mockNanoCreate = NanoEngine.create as jest.Mock;
const mockWebllmCreate = WebLLMEngine.create as jest.Mock;

const TOOL_SPEC = {
  name: "get_expiring_items",
  description: "expiring",
  parametersJson: `{"type":"object"}`,
};

function fakeEngine(replies: string[], id: "nano" | "webllm" = "nano"): LocalEngine {
  return {
    id,
    label: `fake-${id}`,
    chat: jest.fn(async () => replies.shift() ?? `{"answer":"done"}`),
    destroy: jest.fn(),
  };
}

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("useAssistant", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    window.localStorage.clear();
    mockedApi.getAIAvailable.mockResolvedValue(true);
    mockedApi.getAssistantTools.mockResolvedValue([TOOL_SPEC]);
    mockedApi.getAssistantPrompt.mockResolvedValue("You are Dot.");
    mockedApi.callAssistantTool.mockResolvedValue(`[{"name":"milk"}]`);
    mockedApi.askAssistant.mockResolvedValue({
      answer: "server answer",
      toolCalls: [{ name: "get_expiring_items" }],
    });
    mockCap.mockReturnValue("none");
  });

  it("uses the server when no local capability exists", async () => {
    const { result } = renderHook(() => useAssistant(), { wrapper });
    await waitFor(() => expect(result.current.available).toBe(true));
    expect(result.current.status).toBe("unavailable");

    const out = await result.current.ask("hi");
    expect(out).toEqual({
      answer: "server answer",
      tools: ["get_expiring_items"],
      engine: "server",
    });
    expect(mockedApi.askAssistant).toHaveBeenCalledWith("hi");
  });

  it("is unavailable when neither server nor local can run", async () => {
    mockedApi.getAIAvailable.mockResolvedValue(false);
    mockCap.mockReturnValue("none");
    const { result } = renderHook(() => useAssistant(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("unavailable"));
    expect(result.current.available).toBe(false);
  });

  it("auto-creates a ready nano session on first ask", async () => {
    mockCap.mockReturnValue("nano");
    mockNanoAvail.mockResolvedValue("available");
    const engine = fakeEngine([
      `{"toolCalls":[{"name":"get_expiring_items","arguments":{"days":7}}]}`,
      `{"answer":"Your milk expires Friday."}`,
    ]);
    mockNanoCreate.mockResolvedValue(engine);

    const { result } = renderHook(() => useAssistant(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("opt-in"));

    const out = await result.current.ask("what's expiring?");
    expect(out.engine).toBe("nano");
    expect(out.answer).toBe("Your milk expires Friday.");
    expect(out.tools).toEqual(["get_expiring_items"]);
    expect(mockedApi.callAssistantTool).toHaveBeenCalledWith(
      "get_expiring_items",
      `{"days":7}`
    );
    expect(mockedApi.askAssistant).not.toHaveBeenCalled();
    await waitFor(() => expect(result.current.localActive).toBe(true));
  });

  it("falls back to the server when the local engine fails", async () => {
    mockCap.mockReturnValue("nano");
    mockNanoAvail.mockResolvedValue("available");
    const engine = fakeEngine([]);
    (engine.chat as jest.Mock).mockRejectedValue(new Error("model unloaded"));
    mockNanoCreate.mockResolvedValue(engine);

    const { result } = renderHook(() => useAssistant(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("opt-in"));

    const out = await result.current.ask("hi");
    expect(out.engine).toBe("server");
    expect(out.answer).toBe("server answer");
    expect(mockedApi.askAssistant).toHaveBeenCalledWith("hi");
  });

  it("respects server-only mode", async () => {
    window.localStorage.setItem("lena-ai-mode", "server");
    mockCap.mockReturnValue("nano");
    mockNanoAvail.mockResolvedValue("available");
    const { result } = renderHook(() => useAssistant(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("opt-in"));

    await result.current.ask("hi");
    expect(mockedApi.askAssistant).toHaveBeenCalledWith("hi");
    expect(mockNanoCreate).not.toHaveBeenCalled();
  });

  it("enables webllm with download progress via opt-in", async () => {
    mockCap.mockReturnValue("webllm");
    const engine = fakeEngine([`{"answer":"local hi"}`], "webllm");
    mockWebllmCreate.mockImplementation(
      async (_model: string, onProgress?: (p: { fraction: number; text: string }) => void) => {
        onProgress?.({ fraction: 0.5, text: "Fetching weights" });
        onProgress?.({ fraction: 1, text: "Ready" });
        return engine;
      }
    );

    const { result } = renderHook(() => useAssistant(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("opt-in"));

    await act(async () => {
      await result.current.enableLocal();
    });
    await waitFor(() => expect(result.current.status).toBe("ready"));
    expect(result.current.localActive).toBe(true);
    expect(result.current.engineLabel).toBe(engine.label);

    const out = await result.current.ask("hi");
    expect(out.engine).toBe("webllm");
    expect(out.answer).toBe("local hi");
  });
});
