import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import AssistantPage from "@/app/assistant/page";
import { api } from "@/lib/api";

jest.mock("../../../lib/api", () => ({
  api: {
    getAIAvailable: jest.fn(),
    askAssistant: jest.fn(),
  },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

const mockedApi = api as jest.Mocked<typeof api>;

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
    mockedApi.getAIAvailable.mockResolvedValue(true);
    mockedApi.askAssistant.mockResolvedValue({
      answer: "Your milk expires tomorrow.",
      toolCalls: [{ name: "get_expiring_items" }],
    });
  });

  it("shows an info notice when AI is unavailable", async () => {
    mockedApi.getAIAvailable.mockResolvedValue(false);
    renderPage();
    expect(
      await screen.findByText(/isn't configured on this server/i)
    ).toBeInTheDocument();
    expect(mockedApi.askAssistant).not.toHaveBeenCalled();
  });

  it("sends a question and renders the answer with its tool trace", async () => {
    renderPage();
    await screen.findByText("Ask Dot");
    await userEvent.type(screen.getByPlaceholderText("Ask Dot…"), "what is expiring?");
    await userEvent.click(screen.getByRole("button", { name: /send/i }));

    await waitFor(() => screen.getByText("Your milk expires tomorrow."));
    expect(mockedApi.askAssistant).toHaveBeenCalledWith("what is expiring?");
    expect(screen.getByText("what is expiring?")).toBeInTheDocument();
    expect(screen.getByText(/looked up: get_expiring_items/)).toBeInTheDocument();
  });

  it("sends a quick prompt chip", async () => {
    renderPage();
    await screen.findByText("Ask Dot");
    await userEvent.click(screen.getByText("What's in my wine cellar?"));
    await waitFor(() =>
      expect(mockedApi.askAssistant).toHaveBeenCalledWith("What's in my wine cellar?")
    );
    expect(await screen.findByText("Your milk expires tomorrow.")).toBeInTheDocument();
  });

  it("surfaces the server error", async () => {
    const { ApiError } = jest.requireMock("../../../lib/api") as typeof import("../../../lib/api");
    mockedApi.askAssistant.mockRejectedValue(new ApiError(429, "slow down"));
    renderPage();
    await screen.findByText("Ask Dot");
    await userEvent.type(screen.getByPlaceholderText("Ask Dot…"), "hi");
    await userEvent.click(screen.getByRole("button", { name: /send/i }));
    expect(await screen.findByText("slow down")).toBeInTheDocument();
  });
});
