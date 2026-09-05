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
});
