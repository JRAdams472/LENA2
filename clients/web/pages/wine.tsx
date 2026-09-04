import { gql, useMutation, useQuery } from '@apollo/client';
import { useState } from 'react';

const WINE_QUERY = gql`
  query UserBottles {
    userBottles(page: 1, pageSize: 25) {
      items {
        id
        bottle {
          id
          vineyard
          vintageYear
        }
        quantity
        isFavorite
      }
      pageInfo {
        totalCount
      }
    }
  }
`;

const BOTTLES_QUERY = gql`
  query Bottles {
    bottles(page: 1, pageSize: 100) {
      items {
        id
        vineyard
        vintageYear
      }
      pageInfo {
        totalCount
      }
    }
  }
`;

const ADJUST_BOTTLE = gql`
  mutation AdjustUserBottle($bottleId: ID!, $quantity: Int!) {
    adjustUserBottle(bottleId: $bottleId, quantity: $quantity) {
      id
    }
  }
`;

const SET_FAVORITE = gql`
  mutation SetBottleFavorite($bottleId: ID!, $isFavorite: Boolean!) {
    setBottleFavorite(bottleId: $bottleId, isFavorite: $isFavorite) {
      id
    }
  }
`;

type UserBottle = {
  id: string;
  bottle: { id: string; vineyard?: string | null; vintageYear: number };
  quantity: number;
  isFavorite: boolean;
};

type Bottle = {
  id: string;
  vineyard?: string | null;
  vintageYear: number;
};

export default function Wine() {
  const { data, loading, error, refetch } = useQuery(WINE_QUERY);
  const { data: bottlesData } = useQuery(BOTTLES_QUERY);
  const [adjust] = useMutation(ADJUST_BOTTLE, {
    onCompleted: () => refetch(),
  });
  const [setFavorite] = useMutation(SET_FAVORITE, {
    onCompleted: () => refetch(),
  });

  const [form, setForm] = useState({ bottleId: '', quantity: '1' });

  const handleAdjust = (e: React.FormEvent) => {
    e.preventDefault();
    adjust({
      variables: {
        bottleId: form.bottleId,
        quantity: parseInt(form.quantity, 10),
      },
    });
    setForm({ bottleId: '', quantity: '1' });
  };

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const bottles = bottlesData?.bottles?.items ?? [];

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Wine Cellar</h1>

      <form onSubmit={handleAdjust} style={{ marginBottom: '2rem' }}>
        <h2>Add / adjust holding</h2>
        <select
          value={form.bottleId}
          onChange={(e) => setForm({ ...form, bottleId: e.target.value })}
          required
        >
          <option value="">Bottle</option>
          {bottles.map((bottle: Bottle) => (
            <option key={bottle.id} value={bottle.id}>
              {bottle.vineyard ?? 'Unknown'} {bottle.vintageYear}
            </option>
          ))}
        </select>{' '}
        <input
          type="number"
          min={0}
          value={form.quantity}
          onChange={(e) => setForm({ ...form, quantity: e.target.value })}
          required
        />{' '}
        <button type="submit">Save</button>
      </form>

      <ul>
        {data?.userBottles?.items.map((bottle: UserBottle) => (
          <li key={bottle.id}>
            {bottle.bottle.vineyard ?? 'Unknown'} {bottle.bottle.vintageYear} —
            quantity: {bottle.quantity}{' '}
            <button
              onClick={() =>
                setFavorite({
                  variables: {
                    bottleId: bottle.bottle.id,
                    isFavorite: !bottle.isFavorite,
                  },
                })
              }
            >
              {bottle.isFavorite ? '★' : '☆'}
            </button>
          </li>
        ))}
      </ul>
    </main>
  );
}
