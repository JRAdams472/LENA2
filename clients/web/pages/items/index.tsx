import { gql, useMutation, useQuery } from '@apollo/client';
import { useState } from 'react';
import Link from 'next/link';

const ITEMS_QUERY = gql`
  query Items {
    items(page: 1, pageSize: 50) {
      items {
        id
        name
        unit
        brand {
          id
          name
        }
        category {
          id
          name
        }
      }
      pageInfo {
        totalCount
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

const CREATE_ITEM = gql`
  mutation CreateItem($input: CreateItemInput!) {
    createItem(input: $input) {
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

type Item = {
  id: string;
  name: string;
  unit: string;
  brand?: { id: string; name: string } | null;
  category: { id: string; name: string };
};

export default function Items() {
  const { data, loading, error, refetch } = useQuery(ITEMS_QUERY);
  const { data: categoriesData } = useQuery(CATEGORIES_QUERY);
  const { data: brandsData } = useQuery(BRANDS_QUERY);
  const [createItem] = useMutation(CREATE_ITEM, {
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createItem({
      variables: {
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
    setForm({ name: '', unit: '', categoryId: '', brandId: '', upc12: '', upc14: '' });
  };

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const categories = categoriesData?.categories ?? [];
  const brands = brandsData?.brands ?? [];

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Items</h1>

      <form onSubmit={handleSubmit} style={{ marginBottom: '2rem' }}>
        <h2>Create item</h2>
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
          placeholder="UPC-12 (optional)"
          value={form.upc12}
          onChange={(e) => setForm({ ...form, upc12: e.target.value })}
        />{' '}
        <input
          placeholder="UPC-14 (optional)"
          value={form.upc14}
          onChange={(e) => setForm({ ...form, upc14: e.target.value })}
        />{' '}
        <button type="submit">Create</button>
      </form>

      <ul>
        {data?.items?.items.map((item: Item) => (
          <li key={item.id}>
            <Link href={`/items/${item.id}`} legacyBehavior>
              <a>
                {item.name} — {item.unit} ({item.category.name})
                {item.brand ? ` / ${item.brand.name}` : ''}
              </a>
            </Link>
          </li>
        ))}
      </ul>
    </main>
  );
}
