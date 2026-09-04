import { gql, useMutation, useQuery } from '@apollo/client';
import { useState } from 'react';

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

const MEAL_PLANS_QUERY = gql`
  query MealPlans {
    mealPlans(page: 1, pageSize: 25) {
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

const ITEMS_QUERY = gql`
  query Items {
    items(page: 1, pageSize: 100) {
      items {
        id
        name
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

const GENERATE_GROCERY_LIST = gql`
  mutation GenerateGroceryList($mealPlanId: ID!) {
    generateGroceryList(mealPlanId: $mealPlanId) {
      id
    }
  }
`;

const ADD_GROCERY_ITEM = gql`
  mutation AddGroceryItem($input: AddGroceryItemInput!) {
    addGroceryItem(input: $input) {
      id
    }
  }
`;

const DELETE_GROCERY_ITEM = gql`
  mutation DeleteGroceryItem($groceryListItemId: ID!) {
    deleteGroceryItem(groceryListItemId: $groceryListItemId)
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

type MealPlan = {
  id: string;
  name: string;
};

type Item = {
  id: string;
  name: string;
};

export default function GroceryListPage() {
  const { data, loading, error, refetch } = useQuery(GROCERY_LISTS_QUERY);
  const { data: mealPlansData } = useQuery(MEAL_PLANS_QUERY);
  const { data: itemsData } = useQuery(ITEMS_QUERY);
  const [toggle] = useMutation(TOGGLE_GROCERY_ITEM, {
    refetchQueries: ['GroceryLists'],
  });
  const [generate] = useMutation(GENERATE_GROCERY_LIST, {
    onCompleted: () => refetch(),
  });
  const [addGroceryItem] = useMutation(ADD_GROCERY_ITEM, {
    onCompleted: () => refetch(),
  });
  const [deleteGroceryItem] = useMutation(DELETE_GROCERY_ITEM, {
    onCompleted: () => refetch(),
  });

  const [form, setForm] = useState({ mealPlanId: '' });
  const [itemForm, setItemForm] = useState({
    itemId: '',
    manualName: '',
    quantity: '',
    unit: '',
  });

  const handleGenerate = (e: React.FormEvent) => {
    e.preventDefault();
    generate({ variables: { mealPlanId: form.mealPlanId } });
    setForm({ mealPlanId: '' });
  };

  const handleAddItem = (e: React.FormEvent) => {
    e.preventDefault();
    const list = data?.groceryLists?.items?.[0];
    if (!list) return;
    addGroceryItem({
      variables: {
        input: {
          groceryListId: list.id,
          itemId: itemForm.itemId || null,
          manualItemName: itemForm.manualName || null,
          quantity: parseFloat(itemForm.quantity),
          unit: itemForm.unit,
        },
      },
    });
    setItemForm({ itemId: '', manualName: '', quantity: '', unit: '' });
  };

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const list = data?.groceryLists?.items?.[0];
  const plans = mealPlansData?.mealPlans?.items ?? [];
  const items = itemsData?.items?.items ?? [];

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Grocery List</h1>

      <form onSubmit={handleGenerate} style={{ marginBottom: '2rem' }}>
        <h2>Generate from meal plan</h2>
        <select
          value={form.mealPlanId}
          onChange={(e) => setForm({ ...form, mealPlanId: e.target.value })}
          required
        >
          <option value="">Meal plan</option>
          {plans.map((plan: MealPlan) => (
            <option key={plan.id} value={plan.id}>
              {plan.name}
            </option>
          ))}
        </select>{' '}
        <button type="submit">Generate</button>
      </form>

      {list ? (
        <>
          <p>Generated: {list.generatedAt}</p>
          <form onSubmit={handleAddItem} style={{ marginBottom: '1rem' }}>
            <h3>Add item</h3>
            <select
              value={itemForm.itemId}
              onChange={(e) => setItemForm({ ...itemForm, itemId: e.target.value })}
            >
              <option value="">Catalog item</option>
              {items.map((item: Item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>{' '}
            <input
              placeholder="or manual name"
              value={itemForm.manualName}
              onChange={(e) => setItemForm({ ...itemForm, manualName: e.target.value })}
            />{' '}
            <input
              placeholder="Qty"
              value={itemForm.quantity}
              onChange={(e) => setItemForm({ ...itemForm, quantity: e.target.value })}
              required
            />{' '}
            <input
              placeholder="Unit"
              value={itemForm.unit}
              onChange={(e) => setItemForm({ ...itemForm, unit: e.target.value })}
              required
            />{' '}
            <button type="submit">Add</button>
          </form>
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
                </label>{' '}
                <button onClick={() => deleteGroceryItem({ variables: { groceryListItemId: item.id } })}>
                  Delete
                </button>
              </li>
            ))}
          </ul>
        </>
      ) : (
        <p>No grocery list found. Pick a meal plan and generate one.</p>
      )}
    </main>
  );
}
