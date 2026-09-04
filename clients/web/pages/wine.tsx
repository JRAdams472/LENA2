import { gql, useQuery } from '@apollo/client';

const WINE_QUERY = gql`
  query UserBottles {
    userBottles(page: 1, pageSize: 25) {
      items {
        id
        bottle {
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

type UserBottle = {
  id: string;
  bottle: { vineyard?: string | null; vintageYear: number };
  quantity: number;
  isFavorite: boolean;
};

export default function Wine() {
  const { data, loading, error } = useQuery(WINE_QUERY);

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Wine Cellar</h1>
      <ul>
        {data?.userBottles?.items.map((bottle: UserBottle) => (
          <li key={bottle.id}>
            {bottle.bottle.vineyard ?? 'Unknown'} {bottle.bottle.vintageYear} —
            quantity: {bottle.quantity}{' '}
            {bottle.isFavorite ? '★' : ''}
          </li>
        ))}
      </ul>
    </main>
  );
}
