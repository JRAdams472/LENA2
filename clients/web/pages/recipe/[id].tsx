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
      items {
        item {
          id
          name
        }
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
      pageInfo {
        totalCount
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

type Item = {
  id: string;
  name: string;
};

export default function EditRecipe() {
  const router = useRouter();
  const id = router.query.id as string | undefined;

  const { data, loading, error } = useQuery(RECIPE_QUERY, {
    variables: { id },
    skip: !id,
  });
  const { data: itemsData } = useQuery(ITEMS_QUERY);
  const [updateRecipe] = useMutation(UPDATE_RECIPE, {
    onCompleted: () => router.push('/recipes'),
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

  useEffect(() => {
    if (data?.recipe) {
      const r = data.recipe;
      const item = r.items?.[0];
      const step = r.steps?.[0];
      setForm({
        name: r.name ?? '',
        description: r.description ?? '',
        servings: r.servings?.toString() ?? '',
        prepTimeMinutes: r.prepTimeMinutes?.toString() ?? '',
        cookTimeMinutes: r.cookTimeMinutes?.toString() ?? '',
        itemId: item?.item?.id ?? '',
        quantity: item?.quantity?.toString() ?? '',
        unit: item?.unit ?? '',
        stepInstruction: step?.instruction ?? '',
      });
    }
  }, [data]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
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
  };

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const items = itemsData?.items?.items ?? [];

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Edit recipe</h1>
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
        <button type="submit">Update</button>
      </form>
    </main>
  );
}
