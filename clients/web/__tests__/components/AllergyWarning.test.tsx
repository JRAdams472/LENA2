import "@testing-library/jest-dom";
import { render, screen } from "@testing-library/react";
import {
  AllergyWarningChip,
  AllergyWarningsAlert,
  AllergenFlagsLine,
} from "@/app/components/AllergyWarning";
import { AllergenFlag, AllergyWarning } from "@/lib/types";

const allergen = (id: number, name: string) => ({
  allergenID: id,
  name,
  description: null,
  isActive: true,
});

const warning = (over: Partial<AllergyWarning> = {}): AllergyWarning => ({
  member: { userID: 8, displayName: "Ada", firstName: null, lastName: null },
  allergen: allergen(1, "Peanuts"),
  memberKind: "allergy",
  entityKind: "contains",
  ...over,
});

describe("AllergyWarningChip", () => {
  it("renders nothing with no warnings", () => {
    const { container } = render(<AllergyWarningChip warnings={[]} />);
    expect(container).toBeEmptyDOMElement();
    expect(render(<AllergyWarningChip warnings={undefined} />).container).toBeEmptyDOMElement();
  });

  it("shows a severe chip for an allergy + contains conflict", () => {
    render(<AllergyWarningChip warnings={[warning()]} />);
    const chip = screen.getByTestId("allergy-warning-chip");
    expect(chip).toHaveTextContent("1 allergy warning");
    expect(chip.className).toContain("colorError");
  });

  it("stays advisory for dietary or may-contain conflicts", () => {
    const { rerender } = render(
      <AllergyWarningChip warnings={[warning({ memberKind: "dietary" })]} />
    );
    expect(screen.getByTestId("allergy-warning-chip").className).toContain("colorWarning");

    rerender(
      <AllergyWarningChip warnings={[warning({ entityKind: "may_contain" })]} />
    );
    expect(screen.getByTestId("allergy-warning-chip").className).toContain("colorWarning");
  });

  it("pluralizes the count", () => {
    render(
      <AllergyWarningChip
        warnings={[warning(), warning({ allergen: allergen(2, "Dairy") })]}
      />
    );
    expect(screen.getByTestId("allergy-warning-chip")).toHaveTextContent(
      "2 allergy warnings"
    );
  });
});

describe("AllergyWarningsAlert", () => {
  it("renders nothing with no warnings", () => {
    expect(render(<AllergyWarningsAlert warnings={[]} />).container).toBeEmptyDOMElement();
  });

  it("names the member, allergen, and both kinds", () => {
    render(<AllergyWarningsAlert warnings={[warning()]} />);
    expect(screen.getByTestId("allergy-warning-alert")).toHaveTextContent(
      "Ada — Peanuts (allergy; contains)"
    );
  });

  it("uses error severity only for allergy + contains", () => {
    const { rerender } = render(
      <AllergyWarningsAlert warnings={[warning({ entityKind: "may_contain" })]} />
    );
    expect(screen.getByTestId("allergy-warning-alert").className).toContain(
      "colorWarning"
    );
    rerender(<AllergyWarningsAlert warnings={[warning()]} />);
    expect(screen.getByTestId("allergy-warning-alert").className).toContain(
      "colorError"
    );
  });
});

describe("AllergenFlagsLine", () => {
  // An empty flag set means uncurated, not safe — the copy must say so.
  it("renders 'No allergen information' when empty", () => {
    render(<AllergenFlagsLine allergens={[]} />);
    expect(screen.getByTestId("allergen-flags-empty")).toHaveTextContent(
      "No allergen information"
    );
    expect(screen.queryByText(/safe|no allergens/i)).not.toBeInTheDocument();
  });

  it("renders flags with may-contain marked", () => {
    const flags: AllergenFlag[] = [
      { allergen: allergen(1, "Peanuts"), kind: "contains" },
      { allergen: allergen(2, "Dairy"), kind: "may_contain" },
    ];
    render(<AllergenFlagsLine allergens={flags} />);
    expect(screen.getByText("Peanuts")).toBeInTheDocument();
    expect(screen.getByText("Dairy (may contain)")).toBeInTheDocument();
  });
});
