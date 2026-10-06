import {
  ItemDraft,
  StepDraft,
  deltaBadgeLabel,
  describeItemChange,
  describeStepChange,
  draftForLine,
  draftForStep,
  draftFromDelta,
  draftIsDirty,
  itemDraftInputs,
  removeDraftRow,
  stepDraftInputs,
  upsertItemDraft,
  upsertStepDraft,
} from "@/lib/delta";
import type { RecipeDelta, RecipeItem } from "@/lib/types";

const baseDelta: RecipeDelta = {
  recipeDeltaID: 7,
  stale: false,
  orphanedItemCount: 0,
  orphanedStepCount: 0,
  items: [
    {
      recipeDeltaItemID: 11,
      recipeItemID: 42,
      kind: "substitute",
      itemID: 99,
      itemName: "Oat Milk",
      itemBrand: "Planet",
      ingredientID: 5,
      ingredientName: "oat milk",
      quantity: null,
      unitOfMeasure: null,
      unitID: null,
      section: null,
      displayOrder: null,
      notes: null,
      isOptional: null,
      orphaned: false,
    },
    {
      recipeDeltaItemID: 12,
      recipeItemID: null,
      kind: "add",
      itemID: null,
      itemName: null,
      itemBrand: null,
      ingredientID: 8,
      ingredientName: "flaky salt",
      quantity: 1,
      unitOfMeasure: "pinch",
      unitID: 33,
      section: null,
      displayOrder: null,
      notes: null,
      isOptional: true,
      orphaned: false,
    },
  ],
  steps: [
    {
      recipeDeltaStepID: 21,
      stepID: 55,
      kind: "remove",
      stepNumber: null,
      instruction: null,
      durationMinutes: null,
      stepType: null,
      isPassive: null,
      dependsOnStepNumber: null,
      appliance: null,
      orphaned: false,
    },
  ],
  updatedAt: "2026-10-05T12:00:00Z",
};

const line: RecipeItem = {
  recipeID: 1,
  recipeItemID: 42,
  itemID: 3,
  ingredientID: null,
  ingredientName: "whole milk",
  quantity: 2,
  unitOfMeasure: "cup",
  notes: null,
  isOptional: false,
  itemName: "Whole Milk",
  itemBrand: "DairyCo",
};

describe("draftFromDelta", () => {
  it("clones a saved delta into draft rows", () => {
    const draft = draftFromDelta(baseDelta);
    expect(draft.items).toHaveLength(2);
    expect(draft.steps).toHaveLength(1);
    expect(draft.items[0]).toMatchObject({
      recipeItemId: 42,
      kind: "substitute",
      itemId: 99,
      ingredientId: 5,
      orphaned: false,
    });
    expect(draft.items[1]).toMatchObject({
      recipeItemId: null,
      kind: "add",
      unitId: 33,
    });
  });

  it("returns an empty draft for a null delta", () => {
    expect(draftFromDelta(null)).toEqual({ items: [], steps: [] });
    expect(draftFromDelta(undefined)).toEqual({ items: [], steps: [] });
  });
});

describe("draftIsDirty", () => {
  it("is clean right after cloning", () => {
    expect(draftIsDirty(draftFromDelta(baseDelta), baseDelta)).toBe(false);
  });

  it("flags removed rows", () => {
    const draft = draftFromDelta(baseDelta);
    draft.items = removeDraftRow(draft.items, draft.items[0].key);
    expect(draftIsDirty(draft, baseDelta)).toBe(true);
  });

  it("flags field edits", () => {
    const draft = draftFromDelta(baseDelta);
    draft.items[0].quantity = 3;
    expect(draftIsDirty(draft, baseDelta)).toBe(true);
  });

  it("treats an empty draft against a saved delta as dirty", () => {
    expect(draftIsDirty({ items: [], steps: [] }, baseDelta)).toBe(true);
  });

  it("treats an empty draft against no delta as clean", () => {
    expect(draftIsDirty({ items: [], steps: [] }, null)).toBe(false);
  });
});

describe("upsertItemDraft", () => {
  it("enforces one change per anchored line", () => {
    const draft = draftFromDelta(baseDelta);
    const adjust: ItemDraft = {
      key: "line-42",
      recipeItemId: 42,
      kind: "adjust",
      itemId: null,
      itemName: null,
      itemBrand: null,
      ingredientId: null,
      ingredientName: null,
      quantity: 3,
      unitId: null,
      unitName: null,
      section: null,
      displayOrder: null,
      notes: null,
      isOptional: null,
      orphaned: false,
    };
    const items = upsertItemDraft(draft.items, adjust);
    expect(items.filter((i) => i.recipeItemId === 42)).toHaveLength(1);
    expect(items.find((i) => i.recipeItemId === 42)?.kind).toBe("adjust");
    // The unanchored add row survives.
    expect(items.some((i) => i.kind === "add")).toBe(true);
  });

  it("keeps multiple add rows", () => {
    const makeAdd = (key: string): ItemDraft => ({
      key,
      recipeItemId: null,
      kind: "add",
      itemId: null,
      itemName: null,
      itemBrand: null,
      ingredientId: 1,
      ingredientName: "x",
      quantity: 1,
      unitId: null,
      unitName: null,
      section: null,
      displayOrder: null,
      notes: null,
      isOptional: null,
      orphaned: false,
    });
    const items = upsertItemDraft(
      upsertItemDraft([], makeAdd("a")),
      makeAdd("b")
    );
    expect(items).toHaveLength(2);
  });
});

describe("upsertStepDraft", () => {
  it("replaces the existing change for an anchored step", () => {
    const remove: StepDraft = {
      key: "step-55",
      stepId: 55,
      kind: "remove",
      stepNumber: null,
      instruction: null,
      durationMinutes: null,
      stepType: null,
      isPassive: null,
      dependsOnStepNumber: null,
      appliance: null,
      orphaned: false,
    };
    const replace: StepDraft = { ...remove, kind: "replace", instruction: "Stir" };
    const steps = upsertStepDraft(upsertStepDraft([], remove), replace);
    expect(steps).toHaveLength(1);
    expect(steps[0].kind).toBe("replace");
  });
});

describe("draftForLine / draftForStep", () => {
  it("finds the anchored row and ignores adds", () => {
    const draft = draftFromDelta(baseDelta);
    expect(draftForLine(draft.items, 42)?.kind).toBe("substitute");
    expect(draftForLine(draft.items, 999)).toBeNull();
    expect(draftForStep(draft.steps, 55)?.kind).toBe("remove");
  });
});

describe("mutation inputs", () => {
  it("serializes item drafts to the GraphQL input shape", () => {
    const inputs = itemDraftInputs(draftFromDelta(baseDelta).items);
    expect(inputs[0]).toEqual({
      recipeItemId: "42",
      kind: "substitute",
      itemId: "99",
      ingredientId: "5",
      quantity: null,
      unitId: null,
      section: null,
      displayOrder: null,
      notes: null,
      isOptional: null,
    });
    expect(inputs[1]).toMatchObject({
      recipeItemId: null,
      kind: "add",
      unitId: "33",
      isOptional: true,
    });
  });

  it("serializes step drafts to the GraphQL input shape", () => {
    const inputs = stepDraftInputs(draftFromDelta(baseDelta).steps);
    expect(inputs[0]).toEqual({
      stepId: "55",
      kind: "remove",
      stepNumber: null,
      instruction: null,
      durationMinutes: null,
      stepType: null,
      isPassive: null,
      dependsOnStepNumber: null,
      appliance: null,
    });
  });
});

describe("descriptions", () => {
  const draft = draftFromDelta(baseDelta);

  it("describes a substitute with the canonical label", () => {
    expect(describeItemChange(draft.items[0], line)).toBe(
      "Swap whole milk — DairyCo Whole Milk for oat milk — Planet Oat Milk"
    );
  });

  it("describes an add without a base line", () => {
    expect(describeItemChange(draft.items[1], null)).toBe(
      "Add 1 pinch flaky salt"
    );
  });

  it("describes a step remove", () => {
    expect(
      describeStepChange(draft.steps[0], {
        recipeStepID: 55,
        stepNumber: 3,
      } as never)
    ).toBe("Remove step 3");
  });

  it("falls back to the anchor id when the canonical line is gone", () => {
    expect(describeItemChange(draft.items[0], null)).toContain("line 42");
  });
});

describe("deltaBadgeLabel", () => {
  it("labels each effective-row kind", () => {
    expect(deltaBadgeLabel("substitute")).toBe("Swapped");
    expect(deltaBadgeLabel("adjust")).toBe("Adjusted");
    expect(deltaBadgeLabel("replace")).toBe("Edited");
    expect(deltaBadgeLabel("add")).toBe("Added");
    expect(deltaBadgeLabel(null)).toBe("Tweaked");
  });
});
