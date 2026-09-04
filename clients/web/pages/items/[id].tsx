import { gql, useMutation, useQuery } from '@apollo/client';
import { useRouter } from 'next/router';
import { useEffect, useState } from 'react';

const ITEM_QUERY = gql`
  query Item($id: ID!) {
    item(id: $id) {
      id
      name
      unit
      upc12
      upc14
      brand {
        id
        name
      }
      category {
        id
        name
      }
      nutrients {
        nutrient { id name unit }
        amount
      }
      flavors {
        flavor { id name }
        intensity
      }
    }
    categories {
      id
      name
    }
    brands {
      id
      name
    }
    flavorProfiles {
      id
      name
    }
    nutrientTypes {
      id
      name
      unit
    }
  }
`;

const UPDATE_ITEM = gql`
  mutation UpdateItem($id: ID!, $input: UpdateItemInput!) {
    updateItem(id: $id, input: $input) {
      id
      name
    }
  }
`;

const ADD_FOOD_NUTRIENT = gql`
  mutation AddFoodNutrient($input: AddFoodNutrientInput!) {
    addFoodNutrient(input: $input) {
      nutrient { id name unit }
      amount
    }
  }
`;

const REMOVE_FOOD_NUTRIENT = gql`
  mutation RemoveFoodNutrient($itemId: ID!, $nutrientId: ID!) {
    removeFoodNutrient(itemId: $itemId, nutrientId: $nutrientId)
  }
`;

const ADD_FOOD_FLAVOR = gql`
  mutation AddFoodFlavor($input: AddFoodFlavorInput!) {
    addFoodFlavor(input: $input) {
      flavor { id name }
      intensity
    }
  }
`;

const REMOVE_FOOD_FLAVOR = gql`
  mutation RemoveFoodFlavor($itemId: ID!, $flavorId: ID!) {
    removeFoodFlavor(itemId: $itemId, flavorId: $flavorId)
  }
`;

type Option = { id: string; name: string };

export default function EditItem() {
  const router = useRouter();
  const id = router.query.id as string | undefined;
  const { data, loading, error, refetch } = useQuery(ITEM_QUERY, {
    variables: { id },
    skip: !id,
  });
  const [updateItem] = useMutation(UPDATE_ITEM, {
    onCompleted: () => refetch(),
  });
  const [addNutrient] = useMutation(ADD_FOOD_NUTRIENT, {
    onCompleted: () => refetch(),
  });
  const [removeNutrient] = useMutation(REMOVE_FOOD_NUTRIENT, {
    onCompleted: () => refetch(),
  });
  const [addFlavor] = useMutation(ADD_FOOD_FLAVOR, {
    onCompleted: () => refetch(),
  });
  const [removeFlavor] = useMutation(REMOVE_FOOD_FLAVOR, {
    onCompleted: () => refetch(),
  });

  const [form, setForm] = useState({
    name: '',
    unit: '',
    categoryId: '',
    brandId: '',
    upc12: '',
    upc14: '',
  });
  const [nutrientForm, setNutrientForm] = useState({ nutrientId: '', amount: '' });
  const [flavorForm, setFlavorForm] = useState({ flavorId: '', intensity: '3' });

  useEffect(() => {
    if (data?.item) {
      const item = data.item;
      setForm({
        name: item.name ?? '',
        unit: item.unit ?? '',
        categoryId: item.category?.id ?? '',
        brandId: item.brand?.id ?? '',
        upc12: item.upc12 ?? '',
        upc14: item.upc14 ?? '',
      });
    }
  }, [data]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    updateItem({
      variables: {
        id,
        input: {
          name: form.name,
          unit: form.unit,
          categoryId: form.categoryId,
          brandId: form.brandId || null,
          upc12: form.upc12 || null,
          upc14: form.upc14 || null,
        },
      },
    });
  };

  const handleAddNutrient = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !nutrientForm.nutrientId) return;
    addNutrient({
      variables: {
        input: {
          itemId: id,
          nutrientId: nutrientForm.nutrientId,
          amount: parseFloat(nutrientForm.amount),
        },
      },
    });
    setNutrientForm({ nutrientId: '', amount: '' });
  };

  const handleAddFlavor = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !flavorForm.flavorId) return;
    addFlavor({
      variables: {
        input: {
          itemId: id,
          flavorId: flavorForm.flavorId,
          intensity: parseInt(flavorForm.intensity, 10),
        },
      },
    });
    setFlavorForm({ flavorId: '', intensity: '3' });
  };

  if (loading || !id) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const item = data?.item;
  const categories = data?.categories ?? [];
  const brands = data?.brands ?? [];
  const flavorProfiles = data?.flavorProfiles ?? [];
  const nutrientTypes = data?.nutrientTypes ?? [];

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Edit item</h1>
      <form onSubmit={handleSubmit}>
        <input
          placeholder="Name"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          required
        />{' '}
        <input
          placeholder="Unit"
          value={form.unit}
          onChange={(e) => setForm({ ...form, unit: e.target.value })}
          required
        />{' '}
        <select
          value={form.categoryId}
          onChange={(e) => setForm({ ...form, categoryId: e.target.value })}
          required
        >
          <option value="">Category</option>
          {categories.map((c: Option) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>{' '}
        <select
          value={form.brandId}
          onChange={(e) => setForm({ ...form, brandId: e.target.value })}
        >
          <option value="">Brand (optional)</option>
          {brands.map((b: Option) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>{' '}
        <input
          placeholder="UPC-12"
          value={form.upc12}
          onChange={(e) => setForm({ ...form, upc12: e.target.value })}
        />{' '}
        <input
          placeholder="UPC-14"
          value={form.upc14}
          onChange={(e) => setForm({ ...form, upc14: e.target.value })}
        />{' '}
        <button type="submit">Update</button>
      </form>

      <h2 style={{ marginTop: '2rem' }}>Nutrients</h2>
      <ul>
        {(item?.nutrients ?? []).map((n: any) => (
          <li key={n.nutrient.id}>
            {n.nutrient.name}: {n.amount} {n.nutrient.unit}{' '}
            <button onClick={() => removeNutrient({ variables: { itemId: id, nutrientId: n.nutrient.id } })}>
              Remove
            </button>
          </li>
        ))}
      </ul>
      <form onSubmit={handleAddNutrient}>
        <select
          value={nutrientForm.nutrientId}
          onChange={(e) => setNutrientForm({ ...nutrientForm, nutrientId: e.target.value })}
          required
        >
          <option value="">Nutrient</option>
          {nutrientTypes.map((n: Option) => (
            <option key={n.id} value={n.id}>
              {n.name}
            </option>
          ))}
        </select>{' '}
        <input
          placeholder="Amount"
          value={nutrientForm.amount}
          onChange={(e) => setNutrientForm({ ...nutrientForm, amount: e.target.value })}
          required
        />{' '}
        <button type="submit">Add</button>
      </form>

      <h2 style={{ marginTop: '2rem' }}>Flavors</h2>
      <ul>
        {(item?.flavors ?? []).map((f: any) => (
          <li key={f.flavor.id}>
            {f.flavor.name} (intensity {f.intensity}){' '}
            <button onClick={() => removeFlavor({ variables: { itemId: id, flavorId: f.flavor.id } })}>
              Remove
            </button>
          </li>
        ))}
      </ul>
      <form onSubmit={handleAddFlavor}>
        <select
          value={flavorForm.flavorId}
          onChange={(e) => setFlavorForm({ ...flavorForm, flavorId: e.target.value })}
          required
        >
          <option value="">Flavor profile</option>
          {flavorProfiles.map((f: Option) => (
            <option key={f.id} value={f.id}>
              {f.name}
            </option>
          ))}
        </select>{' '}
        <input
          type="number"
          min={1}
          max={5}
          value={flavorForm.intensity}
          onChange={(e) => setFlavorForm({ ...flavorForm, intensity: e.target.value })}
          required
        />{' '}
        <button type="submit">Add</button>
      </form>
    </main>
  );
}
