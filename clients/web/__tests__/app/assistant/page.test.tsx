import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import AssistantPage from "@/app/assistant/page";
import { useAssistant, type AssistantController } from "../../../lib/ai/useAssistant";

jest.mock("../../../lib/ai/useAssistant", () => ({
  useAssistant: jest.fn(),
}));

const mockUseAssistant = useAssistant as jest.Mock;

function controller(over: Partial<AssistantController> = {}): AssistantController {
  return {
    available: true,
    status: "unavailable",
    engineLabel: "Via server",
    localActive: false,
    mode: "auto",
    setServerOnly: jest.fn(),
    enableLocal: jest.fn(async () => {}),
    download: null,
    downloadError: null,
    ask: jest.fn(async () => ({
      answer: "Your milk expires tomorrow.",
      tools: ["get_expiring_items"],
      engine: "server" as const,
    })),
    ...over,
  };
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <AssistantPage />
    </QueryClientProvider>
  );
}

describe("assistant page", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockUseAssistant.mockReturnValue(controller());
  });

  it("shows an info notice when Dot can't run at all", async () => {
    mockUseAssistant.mockReturnValue(controller({ available: false }));
    renderPage();
    expect(
      await screen.findByText(/can't run here/i)
    ).toBeInTheDocument();
  });

  it("sends a question and renders the answer with its tool trace", async () => {
    const c = controller();
    mockUseAssistant.mockReturnValue(c);
    renderPage();
    await screen.findByText("Ask Dot");
    await userEvent.type(screen.getByPlaceholderText("Ask Dot…"), "what is expiring?");
    await userEvent.click(screen.getByRole("button", { name: /send/i }));

    await waitFor(() => screen.getByText("Your milk expires tomorrow."));
    expect(c.ask).toHaveBeenCalledWith("what is expiring?");
    expect(screen.getByText("what is expiring?")).toBeInTheDocument();
    expect(screen.getByText(/looked up: get_expiring_items/)).toBeInTheDocument();
  });

  it("shows the opt-in card and enables local on click", async () => {
    const c = controller({ status: "opt-in" });
    mockUseAssistant.mockReturnValue(c);
    renderPage();
    expect(
      await screen.findByText(/run Dot locally/i)
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /enable/i }));
    expect(c.enableLocal).toHaveBeenCalled();
  });

  it("shows download progress while the model downloads", async () => {
    mockUseAssistant.mockReturnValue(
      controller({
        status: "downloading",
        download: { fraction: 0.42, text: "Fetching weights" },
      })
    );
    renderPage();
    expect(await screen.findByText(/Downloading the on-device model — 42%/)).toBeInTheDocument();
  });

  it("badges local inference and offers a server toggle", async () => {
    const c = controller({ localActive: true, status: "ready" });
    mockUseAssistant.mockReturnValue(c);
    renderPage();
    expect(await screen.findByText("On this device")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /use server instead/i }));
    expect(c.setServerOnly).toHaveBeenCalledWith(true);
  });

  it("surfaces errors from the ask path", async () => {
    const c = controller();
    c.ask = jest.fn(async () => {
      throw new Error("slow down");
    });
    mockUseAssistant.mockReturnValue(c);
    renderPage();
    await screen.findByText("Ask Dot");
    await userEvent.type(screen.getByPlaceholderText("Ask Dot…"), "hi");
    await userEvent.click(screen.getByRole("button", { name: /send/i }));
    expect(await screen.findByText("slow down")).toBeInTheDocument();
  });
});
