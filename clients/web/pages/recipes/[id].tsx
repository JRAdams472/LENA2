import { gql, useMutation, useQuery } from '@apollo/client';
import { useRouter } from 'next/router';
import { useEffect, useState } from 'react';

const RECIPE_QUERY = gql`
  query Recipe($id: ID!) {
    recipe(id: $id) {
      id
      name
      description
      servings
      prepTimeMinutes
      cookTimeMinutes
      isFavorite
      items {
        item { id name }
        quantity
        unit
        notes
        isOptional
      }
      steps {
        stepNumber
        instruction
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

const UPDATE_RECIPE = gql`
  mutation UpdateRecipe($id: ID!, $input: CreateRecipeInput!) {
    updateRecipe(id: $id, input: $input) {
      id
      name
    }
  }
`;

const DELETE_RECIPE = gql`
  mutation DeleteRecipe($id: ID!) {
    deleteRecipe(id: $id)
  }
`;

const SET_RECIPE_FAVORITE = gql`
  mutation SetRecipeFavorite($recipeId: ID!, $isFavorite: Boolean!) {
    setRecipeFavorite(recipeId: $recipeId, isFavorite: $isFavorite)
  }
`;

type Item = { id: string; name: string };

type FormItem = {
  itemId: string;
  quantity: string;
  unit: string;
  notes: string;
  isOptional: boolean;
};

type FormStep = { instruction: string };

export default function EditRecipe() {
  const router = useRouter();
  const id = (router.query.id as string) ?? '';
  const { data, loading, error, refetch } = useQuery(RECIPE_QUERY, {
    variables: { id },
    skip: !id,
  });
  const { data: itemsData } = useQuery(ITEMS_QUERY);
  const [updateRecipe] = useMutation(UPDATE_RECIPE, {
    onCompleted: () => refetch(),
  });
  const [deleteRecipe] = useMutation(DELETE_RECIPE, {
    onCompleted: () => router.push('/recipes'),
  });
  const [setRecipeFavorite] = useMutation(SET_RECIPE_FAVORITE, {
    onCompleted: () => refetch(),
  });

  const [form, setForm] = useState({
    name: '',
    description: '',
    servings: '',
    prepTimeMinutes: '',
    cookTimeMinutes: '',
    items: [{ itemId: '', quantity: '', unit: '', notes: '', isOptional: false } as FormItem],
    steps: [{ instruction: '' } as FormStep],
  });

  useEffect(() => {
    if (!data?.recipe) return;
    const r = data.recipe;
    setForm({
      name: r.name ?? '',
      description: r.description ?? '',
      servings: r.servings?.toString() ?? '',
      prepTimeMinutes: r.prepTimeMinutes?.toString() ?? '',
      cookTimeMinutes: r.cookTimeMinutes?.toString() ?? '',
      items:
        r.items?.length > 0
          ? r.items.map((it: any) => ({
              itemId: it.item?.id ?? '',
              quantity: it.quantity?.toString() ?? '',
              unit: it.unit ?? '',
              notes: it.notes ?? '',
              isOptional: it.isOptional ?? false,
            }))
          : [{ itemId: '', quantity: '', unit: '', notes: '', isOptional: false }],
      steps:
        r.steps?.length > 0
          ? r.steps.map((s: any) => ({ instruction: s.instruction ?? '' }))
          : [{ instruction: '' }],
    });
  }, [data]);

  const addItem = () => {
    setForm({
      ...form,
      items: [...form.items, { itemId: '', quantity: '', unit: '', notes: '', isOptional: false }],
    });
  };
  const removeItem = (index: number) => {
    setForm({ ...form, items: form.items.filter((_, i) => i !== index) });
  };
  const updateItem = (index: number, field: string, value: any) => {
    const items = [...form.items];
    items[index] = { ...items[index], [field]: value };
    setForm({ ...form, items });
  };

  const addStep = () => {
    setForm({ ...form, steps: [...form.steps, { instruction: '' }] });
  };
  const removeStep = (index: number) => {
    setForm({ ...form, steps: form.steps.filter((_, i) => i !== index) });
  };
  const updateStep = (index: number, value: string) => {
    const steps = [...form.steps];
    steps[index] = { instruction: value };
    setForm({ ...form, steps });
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    updateRecipe({
      variables: {
        id,
        input: {
          name: form.name,
          description: form.description || null,
          servings: form.servings ? parseInt(form.servings, 10) : null,
          prepTimeMinutes: form.prepTimeMinutes
            ? parseInt(form.prepTimeMinutes, 10)
            : null,
          cookTimeMinutes: form.cookTimeMinutes
            ? parseInt(form.cookTimeMinutes, 10)
            : null,
          items: form.items
            .filter((it) => it.itemId && it.quantity)
            .map((it) => ({
              itemId: it.itemId,
              quantity: parseFloat(it.quantity),
              unit: it.unit,
              notes: it.notes || null,
              isOptional: it.isOptional,
            })),
          steps: form.steps
            .filter((s) => s.instruction)
            .map((s, index) => ({
              stepNumber: index + 1,
              instruction: s.instruction,
            })),
        },
      },
    });
  };

  if (loading || !id) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const items = itemsData?.items?.items ?? [];
  const recipe = data?.recipe;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Edit Recipe</h1>
      {recipe && (
        <p>
          <button
            onClick={() =>
              setRecipeFavorite({
                variables: { recipeId: recipe.id, isFavorite: !recipe.isFavorite },
              })
            }
          >
            {recipe.isFavorite ? 'Unfavorite' : 'Favorite'}
          </button>{' '}
          <button onClick={() => deleteRecipe({ variables: { id } })}>Delete</button>
        </p>
      )}

      <form onSubmit={handleSubmit}>
        <input
          placeholder="Name"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          required
        />{' '}
        <input
          placeholder="Description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />{' '}
        <input
          placeholder="Servings"
          value={form.servings}
          onChange={(e) => setForm({ ...form, servings: e.target.value })}
        />{' '}
        <input
          placeholder="Prep minutes"
          value={form.prepTimeMinutes}
          onChange={(e) => setForm({ ...form, prepTimeMinutes: e.target.value })}
        />{' '}
        <input
          placeholder="Cook minutes"
          value={form.cookTimeMinutes}
          onChange={(e) => setForm({ ...form, cookTimeMinutes: e.target.value })}
        />

        <h3>Items</h3>
        {form.items.map((it, i) => (
          <div key={i} style={{ marginBottom: '0.5rem' }}>
            <select
              value={it.itemId}
              onChange={(e) => updateItem(i, 'itemId', e.target.value)}
              required
            >
              <option value="">Item</option>
              {items.map((item: Item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>{' '}
            <input
              placeholder="Qty"
              value={it.quantity}
              onChange={(e) => updateItem(i, 'quantity', e.target.value)}
              required
            />{' '}
            <input
              placeholder="Unit"
              value={it.unit}
              onChange={(e) => updateItem(i, 'unit', e.target.value)}
              required
            />{' '}
            <input
              placeholder="Notes"
              value={it.notes}
              onChange={(e) => updateItem(i, 'notes', e.target.value)}
            />{' '}
            <label>
              <input
                type="checkbox"
                checked={it.isOptional}
                onChange={(e) => updateItem(i, 'isOptional', e.target.checked)}
              />
              Optional
            </label>{' '}
            {form.items.length > 1 && (
              <button type="button" onClick={() => removeItem(i)}>
                Remove
              </button>
            )}
          </div>
        ))}
        <button type="button" onClick={addItem}>
          Add item
        </button>

        <h3>Steps</h3>
        {form.steps.map((s, i) => (
          <div key={i} style={{ marginBottom: '0.5rem' }}>
            <span>Step {i + 1}:</span>{' '}
            <input
              placeholder="Instruction"
              value={s.instruction}
              onChange={(e) => updateStep(i, e.target.value)}
              required
              style={{ width: '60%' }}
            />{' '}
            {form.steps.length > 1 && (
              <button type="button" onClick={() => removeStep(i)}>
                Remove
              </button>
            )}
          </div>
        ))}
        <button type="button" onClick={addStep}>
          Add step
        </button>

        <div style={{ marginTop: '1rem' }}>
          <button type="submit">Save</button>
        </div>
      </form>
    </main>
  );
}
