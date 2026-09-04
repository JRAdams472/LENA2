import { gql, useMutation, useQuery } from '@apollo/client';

const GROCERY_LISTS_QUERY = gql`
  query GroceryLists {
    groceryLists(page: 1, pageSize: 1) {
      items {
        id
        generatedAt
        items {
          id
          item { name }
          manualItemName
          quantityNeeded
          unitOfMeasure
          source
          isChecked
        }
      }
    }
  }
`;

const TOGGLE_GROCERY_ITEM = gql`
  mutation ToggleGroceryItem($id: ID!) {
    toggleGroceryItemChecked(groceryListItemId: $id) {
      id
      isChecked
    }
  }
`;

type GroceryItem = {
  id: string;
  item?: { name: string } | null;
  manualItemName?: string | null;
  quantityNeeded: number;
  unitOfMeasure?: string | null;
  source: string;
  isChecked: boolean;
};

export default function GroceryListPage() {
  const { data, loading, error } = useQuery(GROCERY_LISTS_QUERY);
  const [toggle] = useMutation(TOGGLE_GROCERY_ITEM, {
    refetchQueries: ['GroceryLists'],
  });

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const list = data?.groceryLists?.items?.[0];
  if (!list) return <p>No grocery list found.</p>;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Grocery List</h1>
      <p>Generated: {list.generatedAt}</p>
      <ul>
        {list.items.map((item: GroceryItem) => (
          <li key={item.id}>
            <label>
              <input
                type="checkbox"
                checked={item.isChecked}
                onChange={() => toggle({ variables: { id: item.id } })}
              />
              {item.item?.name ?? item.manualItemName ?? 'Unknown'} — {item.quantityNeeded} {item.unitOfMeasure}
            </label>
          </li>
        ))}
      </ul>
    </main>
  );
}
