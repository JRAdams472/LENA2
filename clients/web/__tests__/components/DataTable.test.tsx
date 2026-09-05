import "@testing-library/jest-dom";
import { render, screen, fireEvent } from "@testing-library/react";
import DataTable from "@/app/components/DataTable";

const rows = [
  { itemID: 1, name: "Milk", isFavorite: false },
  { itemID: 2, name: "Cheese", isFavorite: true },
];

describe("DataTable", () => {
  it("renders title, rows, and the create button", () => {
    const onCreate = jest.fn();
    const onEdit = jest.fn();
    const onDelete = jest.fn();

    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={onCreate}
        onEdit={onEdit}
        onDelete={onDelete}
      />
    );

    expect(screen.getByRole("heading", { name: "Items" })).toBeInTheDocument();
    expect(screen.getByText("Milk")).toBeInTheDocument();
    expect(screen.getByText("Cheese")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    expect(onCreate).toHaveBeenCalledTimes(1);
  });

  it("shows no-data message when rows is empty", () => {
    render(
      <DataTable
        title="Empty"
        rows={[]}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
      />
    );

    expect(screen.getByText("No data")).toBeInTheDocument();
  });

  it("renders an error alert", () => {
    const error = new Error("Connection failed");
    render(
      <DataTable
        title="Items"
        rows={[]}
        isLoading={false}
        error={error}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Connection failed");
  });

  it("renders pagination footer with correct defaults and disabled controls", () => {
    const onPageChange = jest.fn();
    const onPageSizeChange = jest.fn();

    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        pagination={{
          pageNumber: 1,
          pageSize: 25,
          totalCount: 50,
          totalPages: 5,
          onPageChange,
          onPageSizeChange,
        }}
      />
    );

    expect(screen.getByRole("combobox")).toHaveTextContent("25");
    expect(screen.getByText(/Page 1 of 5 \(50 total\)/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "<" })).toBeDisabled();
    expect(screen.getByRole("button", { name: ">" })).not.toBeDisabled();

    fireEvent.mouseDown(screen.getByLabelText("Rows per page"));
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(4);
    expect(options.map((o) => o.textContent)).toEqual(["10", "25", "50", "100"]);
  });

  it("disables the next button on the last page", () => {
    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        pagination={{
          pageNumber: 5,
          pageSize: 25,
          totalCount: 50,
          totalPages: 5,
          onPageChange: jest.fn(),
          onPageSizeChange: jest.fn(),
        }}
      />
    );

    expect(screen.getByRole("button", { name: ">" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "<" })).not.toBeDisabled();
  });

  it("calls onPageChange and onPageSizeChange", () => {
    const onPageChange = jest.fn();
    const onPageSizeChange = jest.fn();

    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        pagination={{
          pageNumber: 2,
          pageSize: 25,
          totalCount: 100,
          totalPages: 4,
          onPageChange,
          onPageSizeChange,
        }}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "<" }));
    expect(onPageChange).toHaveBeenCalledWith(1);

    fireEvent.click(screen.getByRole("button", { name: ">" }));
    expect(onPageChange).toHaveBeenCalledWith(3);

    fireEvent.mouseDown(screen.getByLabelText("Rows per page"));
    fireEvent.click(screen.getByRole("option", { name: "50" }));
    expect(onPageSizeChange).toHaveBeenCalledWith(50);
  });

  it("supports the legacy page/pageSize/totalCount props", () => {
    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        page={3}
        pageSize={10}
        totalCount={25}
        onPageChange={jest.fn()}
        onPageSizeChange={jest.fn()}
      />
    );

    expect(screen.getByText(/Page 3 of 3 \(25 total\)/)).toBeInTheDocument();
  });

  it("invokes onEdit and onDelete with the clicked row", () => {
    const onEdit = jest.fn();
    const onDelete = jest.fn();

    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={onEdit}
        onDelete={onDelete}
        fields={[{ key: "name", label: "Name" }]}
      />
    );

    const row = screen.getByText("Milk").closest("tr")!;
    const buttons = row.querySelectorAll("button");
    fireEvent.click(buttons[0]);
    expect(onEdit).toHaveBeenCalledWith(rows[0]);

    fireEvent.click(buttons[1]);
    expect(onDelete).toHaveBeenCalledWith(rows[0]);
  });

  it("sorts rows ascending then descending when a sortable header is clicked", () => {
    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        fields={[{ key: "name", label: "Name", sortable: true }]}
      />
    );

    const getNames = () =>
      screen
        .getAllByRole("row")
        .slice(1)
        .map((r) => r.querySelector("td")!.textContent);

    expect(getNames()).toEqual(["Milk", "Cheese"]);

    fireEvent.click(screen.getByText("Name"));
    expect(getNames()).toEqual(["Cheese", "Milk"]);

    fireEvent.click(screen.getByText("Name"));
    expect(getNames()).toEqual(["Milk", "Cheese"]);
  });

  it("does not render a sort label for non-sortable fields", () => {
    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        fields={[{ key: "name", label: "Name", sortable: false }]}
      />
    );

    expect(screen.queryByRole("button", { name: "Name" })).not.toBeInTheDocument();
  });

  it("hides id and audit columns derived from row keys", () => {
    render(
      <DataTable
        title="Items"
        rows={[
          { itemID: 1, name: "Milk", createdBy: "me", createDate: "today" },
        ]}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
      />
    );

    expect(screen.queryByText("itemID")).not.toBeInTheDocument();
    expect(screen.queryByText("createdBy")).not.toBeInTheDocument();
    expect(screen.queryByText("createDate")).not.toBeInTheDocument();
    expect(screen.getByText("name")).toBeInTheDocument();
  });

  it("renders extraActions per row", () => {
    render(
      <DataTable
        title="Items"
        rows={rows}
        isLoading={false}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
        extraActions={(row) => (
          <button data-testid={`extra-${row.itemID}`}>X</button>
        )}
      />
    );

    expect(screen.getByTestId("extra-1")).toBeInTheDocument();
    expect(screen.getByTestId("extra-2")).toBeInTheDocument();
  });

  it("shows a loading indicator", () => {
    render(
      <DataTable
        title="Items"
        rows={[]}
        isLoading={true}
        error={null}
        onCreate={jest.fn()}
        onEdit={jest.fn()}
        onDelete={jest.fn()}
      />
    );

    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    expect(screen.queryByText("No data")).not.toBeInTheDocument();
  });
});
