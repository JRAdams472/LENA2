import { fmtQty, sizeBadge, stripSize } from "../../lib/format";

describe("fmtQty", () => {
  it("trims trailing zeros and caps at two decimals", () => {
    expect(fmtQty(1.5)).toBe("1.5");
    expect(fmtQty(2)).toBe("2");
    expect(fmtQty(1.234)).toBe("1.23");
  });
});

describe("sizeBadge", () => {
  it("extracts a size token from the name", () => {
    expect(sizeBadge("Cheerios 18 oz box", "each")).toBe("18 oz");
  });

  it("falls back to a meaningful unit, otherwise null", () => {
    expect(sizeBadge("Bananas", "lb")).toBe("lb");
    expect(sizeBadge("Bananas", "each")).toBeNull();
    expect(sizeBadge("Bananas", "")).toBeNull();
  });
});

describe("stripSize", () => {
  it("removes the size token and collapses whitespace", () => {
    expect(stripSize("Ahold Milk 1 gal", "1 gal")).toBe("Ahold Milk");
  });

  it("strips a duplicated brand prefix", () => {
    expect(stripSize("Ahold Ahold Cheese", null, "Ahold")).toBe(
      "Ahold Cheese"
    );
  });

  it("strips punctuation and whitespace at both edges", () => {
    expect(stripSize("- Milk -,", null)).toBe("Milk");
    expect(stripSize(",,  — Cheese —", null)).toBe("Cheese");
  });

  it("does not touch punctuation inside the name", () => {
    // trailing " -" is an edge strip, internal " -" stays
    expect(stripSize("Mac & Cheese - Family Size", "Family Size")).toBe(
      "Mac & Cheese"
    );
    expect(stripSize("Mac - Cheese", null)).toBe("Mac - Cheese");
    expect(stripSize("Milk", null)).toBe("Milk");
  });
});
