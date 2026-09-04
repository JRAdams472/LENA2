import { gql, useMutation, useQuery } from '@apollo/client';
import { useState } from 'react';
import Link from 'next/link';

const RECIPES_QUERY = gql`
  query Recipes {
    recipes(page: 1, pageSize: 25) {
      items {
        id
        name
        description
        servings
      }
      pageInfo {
        totalCount
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
      pageInfo {
        totalCount
      }
    }
  }
`;

const CREATE_RECIPE = gql`
  mutation CreateRecipe($input: CreateRecipeInput!) {
    createRecipe(input: $input) {
      id
      name
    }
  }
`;

type Recipe = {
  id: string;
  name: string;
  description?: string | null;
  servings?: number | null;
};

type Item = {
  id: string;
  name: string;
};

export default function Recipes() {
  const { data, loading, error, refetch } = useQuery(RECIPES_QUERY);
  const { data: itemsData } = useQuery(ITEMS_QUERY);
  const [createRecipe] = useMutation(CREATE_RECIPE, {
    onCompleted: () => refetch(),
  });

  const [form, setForm] = useState({
    name: '',
    description: '',
    servings: '',
    prepTimeMinutes: '',
    cookTimeMinutes: '',
    itemId: '',
    quantity: '',
    unit: '',
    stepInstruction: '',
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createRecipe({
      variables: {
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
          items: [
            {
              itemId: form.itemId,
              quantity: parseFloat(form.quantity),
              unit: form.unit,
              notes: null,
              isOptional: false,
            },
          ],
          steps: [
            {
              stepNumber: 1,
              instruction: form.stepInstruction,
            },
          ],
        },
      },
    });
    setForm({
      name: '',
      description: '',
      servings: '',
      prepTimeMinutes: '',
      cookTimeMinutes: '',
      itemId: '',
      quantity: '',
      unit: '',
      stepInstruction: '',
    });
  };

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const items = itemsData?.items?.items ?? [];

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Recipes</h1>

      <form onSubmit={handleSubmit} style={{ marginBottom: '2rem' }}>
        <h2>Create recipe</h2>
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
        />{' '}
        <select
          value={form.itemId}
          onChange={(e) => setForm({ ...form, itemId: e.target.value })}
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
          value={form.quantity}
          onChange={(e) => setForm({ ...form, quantity: e.target.value })}
          required
        />{' '}
        <input
          placeholder="Unit"
          value={form.unit}
          onChange={(e) => setForm({ ...form, unit: e.target.value })}
          required
        />{' '}
        <input
          placeholder="Step 1"
          value={form.stepInstruction}
          onChange={(e) => setForm({ ...form, stepInstruction: e.target.value })}
          required
        />{' '}
        <button type="submit">Create</button>
      </form>

      <ul>
        {data?.recipes?.items.map((recipe: Recipe) => (
          <li key={recipe.id}>
            <Link href={`/recipe/${recipe.id}`} legacyBehavior>
              <a>
                {recipe.name}
                {recipe.servings ? ` (serves ${recipe.servings})` : ''}
              </a>
            </Link>
          </li>
        ))}
      </ul>
    </main>
  );
}
