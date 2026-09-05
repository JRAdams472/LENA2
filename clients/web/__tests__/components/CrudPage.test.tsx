import "@testing-library/jest-dom";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import CrudPage from "@/app/components/CrudPage";
import { FieldDef } from "@/app/components/CrudDialog";

interface Row {
  id: number;
  name: string;
  isActive: boolean;
}

const fields: FieldDef<Row>[] = [
  { key: "name", label: "Name", sortable: true },
  { key: "isActive", label: "Active", type: "boolean" },
];

const rows: Row[] = [
  { id: 1, name: "Alpha", isActive: true },
  { id: 2, name: "Beta", isActive: false },
];

function renderPage(
  overrides: Partial<Parameters<typeof CrudPage<Row>>[0]> = {}
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const props = {
    title: "Widgets",
    queryKey: ["widgets", Math.random().toString()],
    listFn: jest.fn().mockResolvedValue(rows),
    fields,
    createFn: jest.fn().mockResolvedValue({}),
    updateFn: jest.fn().mockResolvedValue({}),
    deleteFn: jest.fn().mockResolvedValue({}),
    ...overrides,
  };
  render(
    <QueryClientProvider client={queryClient}>
      <CrudPage<Row> {...props} />
    </QueryClientProvider>
  );
  return props;
}

describe("CrudPage", () => {
  beforeEach(() => {
    jest.spyOn(window, "confirm").mockReturnValue(true);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("loads and renders rows", async () => {
    const { listFn } = renderPage();

    await waitFor(() =>
      expect(screen.getByText("Alpha")).toBeInTheDocument()
    );
    expect(screen.getByText("Beta")).toBeInTheDocument();
    expect(listFn).toHaveBeenCalledTimes(1);
  });

  it("shows the list error when loading fails", async () => {
    renderPage({
      listFn: jest.fn().mockRejectedValue(new Error("load exploded")),
    });

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("load exploded")
    );
  });

  it("creates a row via the dialog", async () => {
    const { createFn } = renderPage();

    await waitFor(() => screen.getByText("Alpha"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Gamma" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(createFn).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Gamma" }),
        expect.anything()
      )
    );
    await waitFor(() =>
      expect(screen.queryByText("Create Widgets")).not.toBeInTheDocument()
    );
  });

  it("keeps the dialog open and shows the error when create fails", async () => {
    const { createFn } = renderPage({
      createFn: jest.fn().mockRejectedValue(new Error("create failed")),
    });

    await waitFor(() => screen.getByText("Alpha"));
    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("create failed")
    );
    expect(createFn).toHaveBeenCalledTimes(1);
  });

  it("edits a row via the dialog", async () => {
    const updateFn = jest.fn().mockResolvedValue({});
    renderPage({ updateFn });

    await waitFor(() => screen.getByText("Alpha"));
    const editButtons = screen.getAllByRole("button", { name: "" });
    fireEvent.click(editButtons[0]);

    const nameField = await screen.findByLabelText("Name");
    expect(nameField).toHaveValue("Alpha");

    fireEvent.change(nameField, { target: { value: "Alpha 2" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(updateFn).toHaveBeenCalledWith(
        expect.objectContaining({ id: 1, name: "Alpha 2" }),
        expect.anything()
      )
    );
  });

  it("deletes a row after confirmation", async () => {
    const deleteFn = jest.fn().mockResolvedValue({});
    renderPage({ deleteFn });

    await waitFor(() => screen.getByText("Alpha"));
    const row = screen.getByText("Alpha").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);

    await waitFor(() => expect(deleteFn).toHaveBeenCalledWith(rows[0], expect.anything()));
  });

  it("does not delete when confirmation is cancelled", async () => {
    (window.confirm as jest.Mock).mockReturnValue(false);
    const deleteFn = jest.fn();
    renderPage({ deleteFn });

    await waitFor(() => screen.getByText("Alpha"));
    const row = screen.getByText("Alpha").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);

    expect(window.confirm).toHaveBeenCalled();
    expect(deleteFn).not.toHaveBeenCalled();
  });

  it("shows the delete error on the table when delete fails", async () => {
    renderPage({
      deleteFn: jest.fn().mockRejectedValue(new Error("delete failed")),
    });

    await waitFor(() => screen.getByText("Alpha"));
    const row = screen.getByText("Alpha").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[buttons.length - 1]);

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("delete failed")
    );
  });

  it("switches to the active-only source when toggled", async () => {
    const activeOnlyFn = jest.fn().mockResolvedValue([rows[0]]);
    renderPage({ activeOnlyFn });

    await waitFor(() => screen.getByText("Beta"));
    fireEvent.click(screen.getByLabelText("Active only"));

    await waitFor(() => expect(activeOnlyFn).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.queryByText("Beta")).not.toBeInTheDocument()
    );
    expect(screen.getByText("Alpha")).toBeInTheDocument();
  });

  it("filters rows via the filterBy select", async () => {
    const filterFn = jest.fn().mockResolvedValue([rows[1]]);
    renderPage({
      filterBy: {
        label: "Group",
        optionsFn: jest.fn().mockResolvedValue([{ id: 9, name: "G9" }]),
        filterFn,
      },
    });

    await waitFor(() => screen.getByText("Alpha"));
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(await screen.findByRole("option", { name: "G9" }));

    await waitFor(() => expect(filterFn).toHaveBeenCalledWith(9));
    await waitFor(() =>
      expect(screen.queryByText("Alpha")).not.toBeInTheDocument()
    );
    expect(screen.getByText("Beta")).toBeInTheDocument();
  });
});
