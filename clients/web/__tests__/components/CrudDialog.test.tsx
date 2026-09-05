import "@testing-library/jest-dom";
import { render, screen, fireEvent } from "@testing-library/react";
import CrudDialog, { FieldDef } from "@/app/components/CrudDialog";

interface Row {
  name: string;
  amount: number;
  isActive: boolean;
  startDate: string;
}

const fields: FieldDef<Row>[] = [
  { key: "name", label: "Name" },
  { key: "amount", label: "Amount", type: "number" },
  { key: "isActive", label: "Active", type: "boolean" },
  { key: "startDate", label: "Start date", type: "date" },
];

function renderDialog(
  overrides: Partial<Parameters<typeof CrudDialog<Row>>[0]> = {}
) {
  const onClose = jest.fn();
  const onSave = jest.fn();
  render(
    <CrudDialog<Row>
      open
      title="Edit Row"
      fields={fields}
      values={{}}
      onClose={onClose}
      onSave={onSave}
      {...overrides}
    />
  );
  return { onClose, onSave };
}

describe("CrudDialog", () => {
  it("renders all field types", () => {
    renderDialog();

    expect(screen.getByText("Edit Row")).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    expect(screen.getByLabelText("Amount")).toBeInTheDocument();
    expect(screen.getByLabelText("Active")).toBeInTheDocument();
    expect(screen.getByLabelText("Start date")).toBeInTheDocument();
  });

  it("does not render content when closed", () => {
    renderDialog({ open: false, title: "Hidden" });

    expect(screen.queryByText("Hidden")).not.toBeInTheDocument();
  });

  it("initializes fields from values", () => {
    renderDialog({
      values: { name: "Milk", amount: 3, isActive: true },
    });

    expect(screen.getByLabelText("Name")).toHaveValue("Milk");
    expect(screen.getByLabelText("Amount")).toHaveValue(3);
    expect(screen.getByLabelText("Active")).toBeChecked();
  });

  it("resets the form when values change", () => {
    const { rerender } = render(
      <CrudDialog<Row>
        open
        title="Edit Row"
        fields={fields}
        values={{ name: "First" }}
        onClose={jest.fn()}
        onSave={jest.fn()}
      />
    );
    expect(screen.getByLabelText("Name")).toHaveValue("First");

    rerender(
      <CrudDialog<Row>
        open
        title="Edit Row"
        fields={fields}
        values={{ name: "Second" }}
        onClose={jest.fn()}
        onSave={jest.fn()}
      />
    );
    expect(screen.getByLabelText("Name")).toHaveValue("Second");
  });

  it("submits edited text and number values on Save", () => {
    const { onSave } = renderDialog({ values: { name: "", amount: 0 } });

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Cheese" },
    });
    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "5" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Cheese", amount: 5 })
    );
  });

  it("submits toggled boolean values on Save", () => {
    const { onSave } = renderDialog({ values: { isActive: false } });

    fireEvent.click(screen.getByLabelText("Active"));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({ isActive: true })
    );
  });

  it("submits date values on Save", () => {
    const { onSave } = renderDialog({ values: {} });

    fireEvent.change(screen.getByLabelText("Start date"), {
      target: { value: "2026-09-05" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({ startDate: "2026-09-05" })
    );
  });

  it("keeps an empty number field as an empty string", () => {
    const { onSave } = renderDialog({ values: { amount: 5 } });

    fireEvent.change(screen.getByLabelText("Amount"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({ amount: "" })
    );
  });

  it("calls onClose when Cancel is clicked", () => {
    const { onClose, onSave } = renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onSave).not.toHaveBeenCalled();
  });

  it("renders the error alert", () => {
    renderDialog({ error: new Error("Save failed") });

    expect(screen.getByRole("alert")).toHaveTextContent("Save failed");
  });
});
