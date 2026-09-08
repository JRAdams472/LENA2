import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import ProfilePage from "@/app/profile/page";
import { api } from "@/lib/api";

jest.mock("../../../app/auth/useMe");
jest.mock("../../../lib/api", () => ({
  api: { updateMyProfile: jest.fn() },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
import { useMe } from "@/app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;
const mockedApi = api as jest.Mocked<typeof api>;

const me = {
  userID: 1,
  email: "me@example.com",
  displayName: "Me",
  firstName: "Ada",
  lastName: "Lovelace",
  backupEmail: null,
  role: "member" as const,
  isActive: true,
  isProtected: false,
  lastLoginAt: null,
  externalSubject: null,
  provider: null,
};

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <ProfilePage />
    </QueryClientProvider>
  );
}

describe("profile page", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedUseMe.mockReturnValue({
      me,
      isAdmin: false,
      isLoading: false,
      error: null,
      refetch: jest.fn(),
    });
  });

  it("prefills the form from me", () => {
    renderPage();
    expect(screen.getByLabelText("First name")).toHaveValue("Ada");
    expect(screen.getByLabelText("Last name")).toHaveValue("Lovelace");
    expect(screen.getByLabelText("Backup email")).toHaveValue("");
  });

  it("saves the profile", async () => {
    mockedApi.updateMyProfile.mockResolvedValue({ ...me, backupEmail: "alt@x.com" });
    renderPage();

    await userEvent.type(screen.getByLabelText("Backup email"), "alt@x.com");
    await userEvent.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() =>
      expect(mockedApi.updateMyProfile).toHaveBeenCalledWith({
        firstName: "Ada",
        lastName: "Lovelace",
        backupEmail: "alt@x.com",
      })
    );
    expect(await screen.findByText("Profile saved.")).toBeInTheDocument();
  });
});
