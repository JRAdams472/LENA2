import "@testing-library/jest-dom";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import AllergenFlagsEditor from "@/app/components/AllergenFlagsEditor";
import { api } from "@/lib/api";
import { Allergen, AllergenFlag } from "@/lib/types";

jest.mock("../../lib/api", () => ({
  api: { getAllergens: jest.fn() },
}));

const mockedApi = api as jest.Mocked<typeof api>;

const peanut: Allergen = { allergenID: 1, name: "Peanut", description: null, isActive: true };
const milk: Allergen = { allergenID: 2, name: "Milk", description: null, isActive: true };
const soy: Allergen = { allergenID: 3, name: "Soy", description: null, isActive: true };

const flags: AllergenFlag[] = [
  { allergen: peanut, kind: "contains" },
  { allergen: milk, kind: "may_contain" },
];

function renderEditor(props: {
  flags?: AllergenFlag[];
  onSet?: jest.Mock;
  disabledReason?: string;
}) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <AllergenFlagsEditor onSet={props.onSet ?? jest.fn().mockResolvedValue(undefined)} {...props} />
    </QueryClientProvider>
  );
}

describe("AllergenFlagsEditor", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedApi.getAllergens.mockResolvedValue([peanut, milk, soy]);
  });

  it("renders the disabled reason instead of the editor", () => {
    renderEditor({ flags, disabledReason: "Only the submitter can edit" });
    expect(screen.getByText("Only the submitter can edit")).toBeInTheDocument();
    expect(screen.queryByTestId("allergen-flags-editor")).not.toBeInTheDocument();
  });

  it("lists current flags and offers only unflagged allergens", async () => {
    renderEditor({ flags });
    expect(screen.getByText("Peanut")).toBeInTheDocument();
    expect(screen.getByText("Milk")).toBeInTheDocument();

    const input = screen.getByLabelText("Add allergen");
    fireEvent.mouseDown(input);
    const listbox = await screen.findByRole("listbox");
    // Peanut and Milk are already flagged; only Soy is offered.
    expect(within(listbox).getByText("Soy")).toBeInTheDocument();
    expect(within(listbox).queryByText("Peanut")).not.toBeInTheDocument();
  });

  it("adds a flag as contains", async () => {
    const onSet = jest.fn().mockResolvedValue(undefined);
    renderEditor({ flags: [], onSet });

    const input = screen.getByLabelText("Add allergen");
    fireEvent.mouseDown(input);
    fireEvent.click(await screen.findByText("Soy"));
    fireEvent.click(screen.getByRole("button", { name: "Add" }));

    await waitFor(() => expect(onSet).toHaveBeenCalledWith(3, "contains"));
    await waitFor(() => expect(screen.getByText("Soy")).toBeInTheDocument());
  });

  it("changes a flag kind via the select", async () => {
    const onSet = jest.fn().mockResolvedValue(undefined);
    renderEditor({ flags, onSet });

    fireEvent.mouseDown(screen.getByTestId("flag-kind-1").querySelector('[role="combobox"]')!);
    fireEvent.click(await screen.findByRole("option", { name: "May contain" }));

    await waitFor(() => expect(onSet).toHaveBeenCalledWith(1, "may_contain"));
  });

  it("removes a flag via the delete button", async () => {
    const onSet = jest.fn().mockResolvedValue(undefined);
    renderEditor({ flags, onSet });

    fireEvent.click(screen.getByLabelText("remove Peanut"));
    await waitFor(() => expect(onSet).toHaveBeenCalledWith(1, null));
    await waitFor(() => expect(screen.queryByText("Peanut")).not.toBeInTheDocument());
  });

  it("surfaces onSet failures", async () => {
    const onSet = jest.fn().mockRejectedValue(new Error("server says no"));
    renderEditor({ flags, onSet });

    fireEvent.click(screen.getByLabelText("remove Peanut"));
    await waitFor(() => expect(screen.getByText("server says no")).toBeInTheDocument());
    // The flag stays in the list when the write fails.
    expect(screen.getByText("Peanut")).toBeInTheDocument();
  });
});
