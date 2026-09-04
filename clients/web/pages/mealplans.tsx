import { gql, useQuery } from '@apollo/client';

const MEAL_PLANS_QUERY = gql`
  query MealPlans {
    mealPlans(page: 1, pageSize: 25) {
      items {
        id
        name
        weekStartDate
        isActive
      }
      pageInfo {
        totalCount
      }
    }
  }
`;

type MealPlan = {
  id: string;
  name: string;
  weekStartDate: string;
  isActive: boolean;
};

export default function MealPlans() {
  const { data, loading, error } = useQuery(MEAL_PLANS_QUERY);

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Meal Plans</h1>
      <ul>
        {data?.mealPlans?.items.map((plan: MealPlan) => (
          <li key={plan.id}>
            {plan.name} — week of {plan.weekStartDate}{' '}
            {plan.isActive ? '(active)' : '(inactive)'}
          </li>
        ))}
      </ul>
    </main>
  );
}
