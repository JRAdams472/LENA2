import "@testing-library/jest-dom";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import {
  DeltaBadge,
  ItemTweakEditor,
  RecipeDeltaPanel,
  StepTweakEditor,
} from "@/app/recipes/[id]/recipe-delta";
import { api } from "@/lib/api";
import { DeltaDraft, ItemDraft, StepDraft } from "@/lib/delta";
import { RecipeItem, RecipeStep, Unit } from "@/lib/types";

jest.mock("../../../lib/api", () => ({
  api: {
    getUnits: jest.fn(),
    recipeDeltaEvents: jest.fn(),
    setRecipeDelta: jest.fn(),
    clearRecipeDelta: jest.fn(),
    searchItems: jest.fn(),
    searchIngredients: jest.fn(),
    getOrCreateIngredient: jest.fn(),
    recordSelection: jest.fn(),
  },
}));

const mockedApi = api as jest.Mocked<typeof api>;

const units: Unit[] = [
  { unitID: 1, name: "cup", abbreviation: "c" },
  { unitID: 2, name: "gram", abbreviation: "g" },
] as Unit[];

const line: RecipeItem = {
  recipeItemID: 11,
  quantity: 2,
  unitOfMeasure: "cup",
  isOptional: false,
  notes: "sifted",
  ingredient: { ingredientID: 5, name: "Flour" },
  item: null,
} as unknown as RecipeItem;

const step: RecipeStep = {
  recipeStepID: 21,
  stepNumber: 1,
  instruction: "Mix dry ingredients",
  durationMinutes: 5,
  stepType: "prep",
  isPassive: false,
  appliance: null,
  dependsOnStepNumber: null,
} as RecipeStep;

function renderUI(node: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{node}</QueryClientProvider>);
}

describe("DeltaBadge", () => {
  it("renders the label chip", () => {
    render(<DeltaBadge label="Swapped" />);
    expect(screen.getByText("Swapped")).toBeInTheDocument();
  });
});

describe("ItemTweakEditor", () => {
  it("applies a remove tweak", () => {
    const onApply = jest.fn();
    renderUI(
      <ItemTweakEditor item={line} existing={null} units={units} onApply={onApply} onCancel={jest.fn()} />
    );
    fireEvent.click(screen.getByLabelText("Remove"));
    fireEvent.click(screen.getByRole("button", { name: "Apply tweak" }));
    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ kind: "remove", recipeItemId: 11 })
    );
  });

  it("applies a substitute tweak with the preset ingredient", () => {
    const onApply = jest.fn();
    renderUI(
      <ItemTweakEditor item={line} existing={null} units={units} onApply={onApply} onCancel={jest.fn()} />
    );
    // Default mode is substitute; the line's ingredient is preselected.
    fireEvent.click(screen.getByRole("button", { name: "Apply tweak" }));
    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ kind: "substitute", ingredientId: 5, ingredientName: "Flour" })
    );
  });

  it("applies an adjust tweak with portion, unit, notes, optional", () => {
    const onApply = jest.fn();
    renderUI(
      <ItemTweakEditor item={line} existing={null} units={units} onApply={onApply} onCancel={jest.fn()} />
    );
    fireEvent.click(screen.getByLabelText("Adjust"));
    fireEvent.change(screen.getByLabelText("Portion"), { target: { value: "3" } });
    fireEvent.change(screen.getByLabelText("Notes"), { target: { value: "room temp" } });
    fireEvent.click(screen.getByLabelText("Optional"));
    fireEvent.click(screen.getByRole("button", { name: "Apply tweak" }));
    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: "adjust",
        quantity: 3,
        unitId: 1,
        unitName: "cup",
        notes: "room temp",
        isOptional: true,
      })
    );
  });

  it("cancels without applying", () => {
    const onApply = jest.fn();
    const onCancel = jest.fn();
    renderUI(
      <ItemTweakEditor item={line} existing={null} units={units} onApply={onApply} onCancel={onCancel} />
    );
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalled();
    expect(onApply).not.toHaveBeenCalled();
  });

  it("disables apply when a substitute has nothing selected", () => {
    const bareLine = { ...line, ingredient: null, item: null } as RecipeItem;
    renderUI(
      <ItemTweakEditor item={bareLine} existing={null} units={units} onApply={jest.fn()} onCancel={jest.fn()} />
    );
    expect(screen.getByRole("button", { name: "Apply tweak" })).toBeDisabled();
  });
});

describe("StepTweakEditor", () => {
  it("applies a replace tweak with timing fields", () => {
    const onApply = jest.fn();
    renderUI(
      <StepTweakEditor step={step} existing={null} onApply={onApply} onCancel={jest.fn()} />
    );
    const instruction = screen.getByLabelText("Replacement directions");
    fireEvent.change(instruction, { target: { value: "Whisk instead" } });
    fireEvent.change(screen.getByLabelText("Duration (min)"), { target: { value: "7" } });
    fireEvent.change(screen.getByLabelText("Appliance"), { target: { value: "mixer" } });
    fireEvent.click(screen.getByLabelText("Passive"));
    fireEvent.click(screen.getByRole("button", { name: "Apply tweak" }));
    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: "replace",
        stepId: 21,
        instruction: "Whisk instead",
        durationMinutes: 7,
        isPassive: true,
        appliance: "mixer",
      })
    );
  });

  it("applies a remove tweak", () => {
    const onApply = jest.fn();
    renderUI(
      <StepTweakEditor step={step} existing={null} onApply={onApply} onCancel={jest.fn()} />
    );
    fireEvent.click(screen.getByLabelText("Remove"));
    expect(
      screen.getByText(/won't appear in your household's version/)
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Apply tweak" }));
    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ kind: "remove", stepId: 21 })
    );
  });
});

describe("RecipeDeltaPanel", () => {
  const emptyDraft: DeltaDraft = { items: [], steps: [] };

  beforeEach(() => {
    jest.clearAllMocks();
    mockedApi.getUnits.mockResolvedValue(units);
    mockedApi.recipeDeltaEvents.mockResolvedValue([]);
    mockedApi.searchItems.mockResolvedValue([]);
    mockedApi.searchIngredients.mockResolvedValue([]);
  });

  function renderPanel(overrides: {
    delta?: unknown;
    draft?: DeltaDraft;
    onDraftChange?: jest.Mock;
  } = {}) {
    const onDraftChange = overrides.onDraftChange ?? jest.fn();
    renderUI(
      <RecipeDeltaPanel
        recipeId={7}
        delta={(overrides.delta ?? null) as never}
        draft={overrides.draft ?? emptyDraft}
        onDraftChange={onDraftChange}
        canonicalItems={[line]}
        canonicalSteps={[step]}
      />
    );
    return onDraftChange;
  }

  it("shows the empty state and an empty history", async () => {
    renderPanel();
    expect(screen.getByText(/No tweaks yet/)).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByText("No tweak history yet.")).toBeInTheDocument()
    );
  });

  it("renders draft rows and removes one via the x button", () => {
    const itemDraft: ItemDraft = {
      key: "k1",
      recipeItemId: 11,
      kind: "substitute",
      itemId: null,
      itemName: null,
      itemBrand: null,
      ingredientId: 9,
      ingredientName: "Almond flour",
      quantity: null,
      unitId: null,
      unitName: null,
      section: null,
      displayOrder: null,
      notes: null,
      isOptional: null,
      orphaned: true,
    };
    const stepDraft: StepDraft = {
      key: "k2",
      stepId: 21,
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
    const onDraftChange = renderPanel({
      draft: { items: [itemDraft], steps: [stepDraft] },
    });
    expect(screen.getByText("Swap")).toBeInTheDocument();
    expect(screen.getByText("Remove")).toBeInTheDocument();
    expect(screen.getByText("No longer applies")).toBeInTheDocument();

    const removeButtons = screen.getAllByLabelText("Remove tweak");
    fireEvent.click(removeButtons[0]);
    expect(onDraftChange).toHaveBeenCalledWith({
      items: [],
      steps: [stepDraft],
    });
  });

  it("adds a new step row to the draft", () => {
    const onDraftChange = renderPanel();
    fireEvent.change(screen.getByLabelText("At step"), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText("Step directions"), {
      target: { value: "Fold in cheese" },
    });
    fireEvent.change(screen.getByLabelText("Duration (min)"), { target: { value: "3" } });
    fireEvent.change(screen.getByLabelText("Appliance"), { target: { value: "oven" } });
    fireEvent.click(screen.getByLabelText("Passive"));
    fireEvent.click(screen.getByRole("button", { name: "Add new step" }));

    expect(onDraftChange).toHaveBeenCalledWith(
      expect.objectContaining({
        steps: [
          expect.objectContaining({
            kind: "add",
            stepNumber: 2,
            instruction: "Fold in cheese",
            durationMinutes: 3,
            isPassive: true,
            appliance: "oven",
          }),
        ],
      })
    );
  });

  it("saves the dirty draft through setRecipeDelta", async () => {
    const itemDraft: ItemDraft = {
      key: "k1",
      recipeItemId: 11,
      kind: "remove",
      itemId: null,
      itemName: null,
      itemBrand: null,
      ingredientId: null,
      ingredientName: null,
      quantity: null,
      unitId: null,
      unitName: null,
      section: null,
      displayOrder: null,
      notes: null,
      isOptional: null,
      orphaned: false,
    };
    mockedApi.setRecipeDelta.mockResolvedValue({
      recipeDeltaID: 1,
      stale: false,
      orphanedItemCount: 0,
      orphanedStepCount: 0,
      updatedAt: "2025-01-01T00:00:00Z",
      items: [],
      steps: [],
    });
    renderPanel({ draft: { items: [itemDraft], steps: [] } });

    fireEvent.click(screen.getByRole("button", { name: "Save tweaks" }));
    await waitFor(() => expect(mockedApi.setRecipeDelta).toHaveBeenCalled());
    expect(mockedApi.setRecipeDelta.mock.calls[0][0]).toBe(7);
  });

  it("clears all tweaks after confirmation", async () => {
    mockedApi.clearRecipeDelta.mockResolvedValue(undefined);
    const confirmSpy = jest.spyOn(window, "confirm").mockReturnValue(true);
    renderPanel({ delta: { recipeDeltaID: 1, items: [], steps: [] } });
    fireEvent.click(screen.getByRole("button", { name: "Clear all tweaks" }));
    await waitFor(() => expect(mockedApi.clearRecipeDelta).toHaveBeenCalledWith(7));
    confirmSpy.mockRestore();
  });

  it("describes history events", async () => {
    mockedApi.recipeDeltaEvents.mockResolvedValue([
      {
        recipeDeltaEventID: 1,
        event: "set",
        actor: "a@x.com",
        itemCount: 2,
        stepCount: 1,
        detail: "",
        createdAt: "2025-01-01T00:00:00Z",
      },
      {
        recipeDeltaEventID: 2,
        event: "clear",
        actor: "b@x.com",
        itemCount: 0,
        stepCount: 0,
        detail: "",
        createdAt: "2025-01-02T00:00:00Z",
      },
      {
        recipeDeltaEventID: 3,
        event: "acknowledge",
        actor: "c@x.com",
        itemCount: 0,
        stepCount: 0,
        detail: "",
        createdAt: "2025-01-03T00:00:00Z",
      },
    ]);
    renderPanel();
    await waitFor(() =>
      expect(
        screen.getByText("a@x.com saved the tweak set — 2 lines + 1 step")
      ).toBeInTheDocument()
    );
    expect(screen.getByText("b@x.com cleared all tweaks")).toBeInTheDocument();
    expect(
      screen.getByText("c@x.com marked the tweaks reviewed")
    ).toBeInTheDocument();
    expect(screen.getByText("Saved")).toBeInTheDocument();
    expect(screen.getByText("Cleared")).toBeInTheDocument();
    expect(screen.getByText("Reviewed")).toBeInTheDocument();
  });
});
