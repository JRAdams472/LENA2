import { gql, useQuery } from '@apollo/client';

const RECIPES_QUERY = gql`
  query Recipes {
    recipes(page: 1, pageSize: 25) {
      items {
        id
        name
        description
        servings
        prepTimeMinutes
        cookTimeMinutes
      }
      pageInfo {
        totalCount
      }
    }
  }
`;

type Recipe = {
  id: string;
  name: string;
  description?: string | null;
  servings?: number | null;
  prepTimeMinutes?: number | null;
  cookTimeMinutes?: number | null;
};

export default function Recipes() {
  const { data, loading, error } = useQuery(RECIPES_QUERY);

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Recipes</h1>
      <ul>
        {data?.recipes?.items.map((recipe: Recipe) => (
          <li key={recipe.id}>
            {recipe.name}
            {recipe.description ? ` — ${recipe.description}` : ''}
            {recipe.servings ? ` (serves ${recipe.servings})` : ''}
          </li>
        ))}
      </ul>
    </main>
  );
}
