import { gql, useQuery } from '@apollo/client';

const DASHBOARD_QUERY = gql`
  query Dashboard {
    me {
      id
      email
      displayName
    }
    userItems(page: 1, pageSize: 10) {
      items {
        id
        item {
          name
        }
        currentQty
      }
      pageInfo {
        totalCount
      }
    }
  }
`;

type DashboardData = {
  me: {
    id: string;
    email: string;
    displayName?: string | null;
  };
  userItems: {
    items: Array<{
      id: string;
      item: { name: string };
      currentQty: number;
    }>;
    pageInfo: { totalCount: number };
  };
};

export default function Home() {
  const { data, loading, error } = useQuery<DashboardData>(DASHBOARD_QUERY);

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>LENA</h1>
      <p>{data?.me.displayName ?? data?.me.email}</p>

      <h2>Pantry</h2>
      <ul>
        {data?.userItems.items.map((ui) => (
          <li key={ui.id}>
            {ui.item.name} — {ui.currentQty}
          </li>
        ))}
      </ul>
    </main>
  );
}
