import { gql, useMutation, useQuery } from '@apollo/client';
import { useRouter } from 'next/router';
import { useEffect, useState } from 'react';

const MEAL_PLAN_QUERY = gql`
  query MealPlan($id: ID!) {
    mealPlan(id: $id) {
      id
      name
      weekStartDate
      isActive
      slots {
        id
        dayOfWeek
        mealType
        servings
        replacementNote
        recipe {
          id
          name
        }
        items {
          id
          item {
            id
            name
          }
          quantity
          unit
          isFromRecipe
        }
      }
    }
  }
`;

const RECIPES_QUERY = gql`
  query Recipes {
    recipes(page: 1, pageSize: 100) {
      items {
        id
        name
      }
    }
  }
`;

const ITEMS_QUERY = gql`
  query Items {
    items(page: 1, pageSize: 100) {
      items {
        id
        name
      }
    }
  }
`;

const UPDATE_MEAL_PLAN = gql`
  mutation UpdateMealPlan($id: ID!, $input: CreateMealPlanInput!) {
    updateMealPlan(id: $id, input: $input) {
      id
      name
    }
  }
`;

const ADD_MEAL_SLOT = gql`
  mutation AddMealSlot($input: AddMealSlotInput!) {
    addMealSlot(input: $input) {
      id
    }
  }
`;

const REMOVE_MEAL_SLOT = gql`
  mutation RemoveMealSlot($slotId: ID!) {
    removeMealSlot(slotId: $slotId)
  }
`;

const ADD_MEAL_SLOT_ITEM = gql`
  mutation AddMealSlotItem($input: AddMealSlotItemInput!) {
    addMealSlotItem(input: $input) {
      id
    }
  }
`;

const REMOVE_MEAL_SLOT_ITEM = gql`
  mutation RemoveMealSlotItem($slotItemId: ID!) {
    removeMealSlotItem(slotItemId: $slotItemId)
  }
`;

export default function EditMealPlan() {
  const router = useRouter();
  const id = router.query.id as string | undefined;

  const { data, loading, error, refetch } = useQuery(MEAL_PLAN_QUERY, {
    variables: { id },
    skip: !id,
  });
  const { data: recipesData } = useQuery(RECIPES_QUERY);
  const { data: itemsData } = useQuery(ITEMS_QUERY);

  const [updateMealPlan] = useMutation(UPDATE_MEAL_PLAN, {
    onCompleted: () => refetch(),
  });
  const [addMealSlot] = useMutation(ADD_MEAL_SLOT, {
    onCompleted: () => refetch(),
  });
  const [removeMealSlot] = useMutation(REMOVE_MEAL_SLOT, {
    onCompleted: () => refetch(),
  });
  const [addMealSlotItem] = useMutation(ADD_MEAL_SLOT_ITEM, {
    onCompleted: () => refetch(),
  });
  const [removeMealSlotItem] = useMutation(REMOVE_MEAL_SLOT_ITEM, {
    onCompleted: () => refetch(),
  });

  const [form, setForm] = useState({
    name: '',
    weekStartDate: '',
    weekStartDayOfWeek: '0',
  });
  const [slotForm, setSlotForm] = useState({
    dayOfWeek: '0',
    mealType: '',
    recipeId: '',
    servings: '',
    replacementNote: '',
  });
  const [itemForm, setItemForm] = useState<Record<string, { itemId: string; quantity: string; unit: string }>>({});

  useEffect(() => {
    if (data?.mealPlan) {
      const p = data.mealPlan;
      setForm({
        name: p.name ?? '',
        weekStartDate: p.weekStartDate ?? '',
        weekStartDayOfWeek: p.weekStartDayOfWeek?.toString() ?? '0',
      });
    }
  }, [data]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    updateMealPlan({
      variables: {
        id,
        input: {
          name: form.name,
          weekStartDate: form.weekStartDate,
          weekStartDayOfWeek: parseInt(form.weekStartDayOfWeek, 10),
        },
      },
    });
  };

  const handleAddSlot = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    addMealSlot({
      variables: {
        input: {
          mealPlanId: id,
          dayOfWeek: parseInt(slotForm.dayOfWeek, 10),
          mealType: slotForm.mealType,
          recipeId: slotForm.recipeId || null,
          servings: slotForm.servings ? parseInt(slotForm.servings, 10) : null,
          replacementNote: slotForm.replacementNote || null,
        },
      },
    });
    setSlotForm({ dayOfWeek: '0', mealType: '', recipeId: '', servings: '', replacementNote: '' });
  };

  const handleAddSlotItem = (slotId: string) => {
    const f = itemForm[slotId] ?? { itemId: '', quantity: '', unit: '' };
    if (!f.itemId || !f.quantity) return;
    addMealSlotItem({
      variables: {
        input: {
          slotId,
          itemId: f.itemId,
          quantity: parseFloat(f.quantity),
          unit: f.unit,
          isFromRecipe: false,
        },
      },
    });
    setItemForm({ ...itemForm, [slotId]: { itemId: '', quantity: '', unit: '' } });
  };

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const recipes = recipesData?.recipes?.items ?? [];
  const items = itemsData?.items?.items ?? [];
  const slots = data?.mealPlan?.slots ?? [];

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Edit meal plan</h1>
      <form onSubmit={handleSubmit}>
        <input
          placeholder="Name"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          required
        />{' '}
        <input
          type="date"
          value={form.weekStartDate}
          onChange={(e) => setForm({ ...form, weekStartDate: e.target.value })}
          required
        />{' '}
        <input
          type="number"
          min={0}
          max={6}
          placeholder="Week start day (0-6)"
          value={form.weekStartDayOfWeek}
          onChange={(e) => setForm({ ...form, weekStartDayOfWeek: e.target.value })}
        />{' '}
        <button type="submit">Update</button>
      </form>

      <h2 style={{ marginTop: '2rem' }}>Slots</h2>
      <form onSubmit={handleAddSlot}>
        <input
          type="number"
          min={0}
          max={6}
          placeholder="Day (0-6)"
          value={slotForm.dayOfWeek}
          onChange={(e) => setSlotForm({ ...slotForm, dayOfWeek: e.target.value })}
          required
        />{' '}
        <input
          placeholder="Meal type"
          value={slotForm.mealType}
          onChange={(e) => setSlotForm({ ...slotForm, mealType: e.target.value })}
          required
        />{' '}
        <select
          value={slotForm.recipeId}
          onChange={(e) => setSlotForm({ ...slotForm, recipeId: e.target.value })}
        >
          <option value="">Recipe (optional)</option>
          {recipes.map((r: { id: string; name: string }) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </select>{' '}
        <input
          type="number"
          placeholder="Servings"
          value={slotForm.servings}
          onChange={(e) => setSlotForm({ ...slotForm, servings: e.target.value })}
        />{' '}
        <input
          placeholder="Note"
          value={slotForm.replacementNote}
          onChange={(e) => setSlotForm({ ...slotForm, replacementNote: e.target.value })}
        />{' '}
        <button type="submit">Add slot</button>
      </form>

      <ul>
        {slots.map((slot: any) => (
          <li key={slot.id} style={{ marginBottom: '1rem' }}>
            <strong>
              Day {slot.dayOfWeek} - {slot.mealType}
            </strong>{' '}
            {slot.servings ? `(serves ${slot.servings})` : ''}{' '}
            {slot.recipe ? `Recipe: ${slot.recipe.name}` : ''}
            {slot.replacementNote ? ` — ${slot.replacementNote}` : ''}{' '}
            <button onClick={() => removeMealSlot({ variables: { slotId: slot.id } })}>Remove</button>
            <ul>
              {slot.items.map((it: any) => (
                <li key={it.id}>
                  {it.item?.name ?? 'From recipe'} {it.quantity} {it.unit}{' '}
                  {it.isFromRecipe ? '(recipe)' : ''}{' '}
                  <button onClick={() => removeMealSlotItem({ variables: { slotItemId: it.id } })}>Remove</button>
                </li>
              ))}
            </ul>
            <div>
              <select
                value={itemForm[slot.id]?.itemId ?? ''}
                onChange={(e) =>
                  setItemForm({
                    ...itemForm,
                    [slot.id]: { ...itemForm[slot.id], itemId: e.target.value },
                  })
                }
              >
                <option value="">Item</option>
                {items.map((it: { id: string; name: string }) => (
                  <option key={it.id} value={it.id}>
                    {it.name}
                  </option>
                ))}
              </select>{' '}
              <input
                placeholder="Qty"
                value={itemForm[slot.id]?.quantity ?? ''}
                onChange={(e) =>
                  setItemForm({
                    ...itemForm,
                    [slot.id]: { ...itemForm[slot.id], quantity: e.target.value },
                  })
                }
              />{' '}
              <input
                placeholder="Unit"
                value={itemForm[slot.id]?.unit ?? ''}
                onChange={(e) =>
                  setItemForm({
                    ...itemForm,
                    [slot.id]: { ...itemForm[slot.id], unit: e.target.value },
                  })
                }
              />{' '}
              <button onClick={() => handleAddSlotItem(slot.id)}>Add item</button>
            </div>
          </li>
        ))}
      </ul>
    </main>
  );
}
