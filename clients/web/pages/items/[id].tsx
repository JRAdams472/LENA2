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
    }
  }
`;

const CATEGORIES_QUERY = gql`
  query Categories {
    categories {
      id
      name
    }
  }
`;

const BRANDS_QUERY = gql`
  query Brands {
    brands {
      id
      name
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

type Category = {
  id: string;
  name: string;
};

type Brand = {
  id: string;
  name: string;
};

export default function EditItem() {
  const router = useRouter();
  const id = router.query.id as string | undefined;

  const { data, loading, error } = useQuery(ITEM_QUERY, {
    variables: { id },
    skip: !id,
  });
  const { data: categoriesData } = useQuery(CATEGORIES_QUERY);
  const { data: brandsData } = useQuery(BRANDS_QUERY);
  const [updateItem] = useMutation(UPDATE_ITEM, {
    onCompleted: () => router.push('/items'),
  });

  const [form, setForm] = useState({
    name: '',
    unit: '',
    categoryId: '',
    brandId: '',
    upc12: '',
    upc14: '',
  });

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

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const categories = categoriesData?.categories ?? [];
  const brands = brandsData?.brands ?? [];

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
          {categories.map((c: Category) => (
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
          {brands.map((b: Brand) => (
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
    </main>
  );
}
