import * as aiSuggest from "@/lib/ai/suggest";
import * as engineStore from "@/lib/ai/engineStore";
import { api } from "@/lib/api";
import type { EngineMessage, LocalEngine } from "@/lib/ai/types";

jest.mock("../../../lib/ai/engineStore", () => ({
  engineForSuggestions: jest.fn(),
}));
jest.mock("../../../lib/api", () => {
  const actual = jest.requireActual("../../../lib/api");
  return {
    ...actual,
    api: {
      prepareAssistantRequest: jest.fn(),
      suggestMeals: jest.fn(),
      suggestEventFixes: jest.fn(),
      suggestPairings: jest.fn(),
      suggestCocktails: jest.fn(),
      getRecipe: jest.fn(),
    },
  };
});

const mockedEngine = engineStore.engineForSuggestions as jest.Mock;
const mockedApi = api as unknown as {
  prepareAssistantRequest: jest.Mock;
  suggestMeals: jest.Mock;
  suggestEventFixes: jest.Mock;
  suggestPairings: jest.Mock;
  suggestCocktails: jest.Mock;
  getRecipe: jest.Mock;
};

function fakeEngine(replies: string[]): LocalEngine & { requests: string[][] } {
  const requests: string[][] = [];
  return {
    id: "nano",
    label: "fake",
    requests,
    chat: async (msgs: EngineMessage[]) => {
      requests.push(msgs.map((m) => m.content));
      return replies.length ? replies.shift()! : '{"suggestions":[]}';
    },
    destroy: () => {},
  };
}

const MEALS_CTX = {
  plan: { mealPlanId: 5, slots: [] },
  occupied: [],
  candidates: [{ id: 42, name: "Pasta" }],
  maxSuggestions: 6,
};

const prepared = (ctx: object) => ({
  prompt: "You are helpful.",
  contextJson: JSON.stringify(ctx),
  outputSchemaJson: '{"type":"object"}',
});

beforeEach(() => jest.resetAllMocks());

describe("suggestMeals", () => {
  it("runs locally, validates picks, and hydrates recipes", async () => {
    const engine = fakeEngine([
      '{"suggestions":[{"recipeId":42,"dayOfWeek":2,"mealType":"dinner","reason":"quick","usesExpiring":["milk"]},{"recipeId":99,"dayOfWeek":0,"mealType":"lunch","reason":"bogus"}]}',
    ]);
    mockedEngine.mockResolvedValue(engine);
    mockedApi.prepareAssistantRequest.mockResolvedValue(prepared(MEALS_CTX));
    mockedApi.getRecipe.mockResolvedValue({ recipeID: 42, recipeName: "Pasta" });

    const out = await aiSuggest.suggestMeals(5);
    expect(mockedApi.suggestMeals).not.toHaveBeenCalled();
    expect(mockedApi.getRecipe).toHaveBeenCalledWith(42);
    expect(out).toHaveLength(1);
    expect(out[0]).toMatchObject({
      dayOfWeek: 2,
      mealType: 2, // dinner
      reason: "quick",
      usesExpiringItems: ["milk"],
      recipe: { recipeID: 42 },
    });
  });

  it("returns [] when the server reports nothing to prepare", async () => {
    mockedEngine.mockResolvedValue(fakeEngine([]));
    mockedApi.prepareAssistantRequest.mockResolvedValue(null);
    const out = await aiSuggest.suggestMeals(5);
    expect(out).toEqual([]);
    expect(mockedApi.suggestMeals).not.toHaveBeenCalled();
  });

  it("falls back to the server when local generation fails", async () => {
    mockedEngine.mockResolvedValue(fakeEngine(["not json", "still not json"]));
    mockedApi.prepareAssistantRequest.mockResolvedValue(prepared(MEALS_CTX));
    mockedApi.suggestMeals.mockResolvedValue([{ recipe: { recipeID: 1 } }]);
    const out = await aiSuggest.suggestMeals(5);
    expect(mockedApi.suggestMeals).toHaveBeenCalledWith(5, 6);
    expect(out).toHaveLength(1);
  });

  it("goes straight to the server when no local engine is ready", async () => {
    mockedEngine.mockResolvedValue(null);
    mockedApi.suggestMeals.mockResolvedValue([]);
    await aiSuggest.suggestMeals(5);
    expect(mockedApi.suggestMeals).toHaveBeenCalledWith(5, 6);
    expect(mockedApi.prepareAssistantRequest).not.toHaveBeenCalled();
  });
});

describe("suggestEventFixes", () => {
  it("runs locally and validates against the timeline", async () => {
    const ctx = {
      slotGranularityMinutes: 15,
      recipes: [{ eventRecipeId: 3, name: "Turkey", steps: [{ step: 1 }] }],
    };
    mockedEngine.mockResolvedValue(
      fakeEngine([
        '{"fixes":[{"eventRecipeId":3,"action":"shift_serve","minutes":30,"reason":"frees oven"},{"eventRecipeId":3,"action":"shift_serve","minutes":17,"reason":"bad grain"}]}',
      ])
    );
    mockedApi.prepareAssistantRequest.mockResolvedValue(prepared(ctx));
    const out = await aiSuggest.suggestEventFixes(9);
    expect(out).toHaveLength(1);
    expect(out[0]).toMatchObject({ recipeName: "Turkey", minutes: 30 });
  });
});

describe("suggestPairings", () => {
  it("runs locally with cellar validation", async () => {
    const ctx = { cellar: [{ bottleId: 7, vineyard: "V", vintageYear: 2020 }] };
    mockedEngine.mockResolvedValue(
      fakeEngine(['{"pairings":[{"bottleId":7,"reason":"acidity"}]}'])
    );
    mockedApi.prepareAssistantRequest.mockResolvedValue(prepared(ctx));
    const out = await aiSuggest.suggestPairings(4);
    expect(out).toEqual([
      { bottleId: 7, name: "V 2020", reason: "acidity", inCellar: true },
    ]);
  });
});

describe("suggestCocktails", () => {
  it("runs locally and hydrates cocktail recipes", async () => {
    const ctx = { cocktails: [{ id: 11 }] };
    mockedEngine.mockResolvedValue(
      fakeEngine(['{"suggestions":[{"recipeId":11,"reason":"citrus"}]}'])
    );
    mockedApi.prepareAssistantRequest.mockResolvedValue(prepared(ctx));
    mockedApi.getRecipe.mockResolvedValue({ recipeID: 11, recipeName: "Mule" });
    const out = await aiSuggest.suggestCocktails(true);
    expect(mockedApi.suggestCocktails).not.toHaveBeenCalled();
    expect(out[0]).toMatchObject({
      recipe: { recipeID: 11 },
      reason: "citrus",
      missingIngredients: [],
    });
  });
});
