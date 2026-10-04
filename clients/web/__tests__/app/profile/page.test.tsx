import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import ProfilePage from "@/app/profile/page";
import { api } from "@/lib/api";

jest.mock("../../../app/auth/useMe");
jest.mock("../../../lib/api", () => ({
  api: {
    updateMyProfile: jest.fn(),
    getAllergens: jest.fn(),
    getMyAllergies: jest.fn(),
    setMyAllergy: jest.fn(),
  },
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
    birthdate: null,
  role: "member" as const,
  isActive: true,
  isProtected: false,
  lastLoginAt: null,
  externalSubject: null,
  provider: null,
  isSearchable: true,
  household: null,
};

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <ProfilePage />
    </QueryClientProvider>
  );
}

const allergen = (id: number, name: string) => ({
  allergenID: id,
  name,
  description: null,
  isActive: true,
});

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
    mockedApi.getAllergens.mockResolvedValue([
      allergen(1, "Peanuts"),
      allergen(2, "Gluten"),
      { ...allergen(3, "Retired"), isActive: false },
    ]);
    mockedApi.getMyAllergies.mockResolvedValue([]);
    mockedApi.setMyAllergy.mockResolvedValue(undefined);
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
        birthdate: "",
        isSearchable: true,
      })
    );
    expect(await screen.findByText("Profile saved.")).toBeInTheDocument();
  });

  it("lists active allergens and skips inactive ones", async () => {
    renderPage();
    expect(await screen.findByText("Peanuts")).toBeInTheDocument();
    expect(screen.getByText("Gluten")).toBeInTheDocument();
    expect(screen.queryByText("Retired")).not.toBeInTheDocument();
  });

  it("marks the current record's kind", async () => {
    mockedApi.getMyAllergies.mockResolvedValue([
      { allergen: allergen(1, "Peanuts"), kind: "allergy" },
    ]);
    renderPage();

    const group = await screen.findByTestId("allergy-kind-1");
    await waitFor(() =>
      expect(within(group).getByRole("button", { name: "Allergy" })).toHaveAttribute(
        "aria-pressed",
        "true"
      )
    );
    expect(within(group).getByRole("button", { name: "Dietary" })).toHaveAttribute(
      "aria-pressed",
      "false"
    );
  });

  it("sets a record when a kind is picked", async () => {
    renderPage();
    const group = await screen.findByTestId("allergy-kind-2");
    await userEvent.click(within(group).getByRole("button", { name: "Dietary" }));

    await waitFor(() =>
      expect(mockedApi.setMyAllergy).toHaveBeenCalledWith(2, "dietary", true)
    );
  });

  it("clears the record when None is picked", async () => {
    mockedApi.getMyAllergies.mockResolvedValue([
      { allergen: allergen(1, "Peanuts"), kind: "allergy" },
    ]);
    renderPage();
    const group = await screen.findByTestId("allergy-kind-1");
    await waitFor(() =>
      expect(within(group).getByRole("button", { name: "Allergy" })).toHaveAttribute(
        "aria-pressed",
        "true"
      )
    );

    await userEvent.click(within(group).getByRole("button", { name: "None" }));

    await waitFor(() =>
      expect(mockedApi.setMyAllergy).toHaveBeenCalledWith(1, "allergy", false)
    );
  });
});
