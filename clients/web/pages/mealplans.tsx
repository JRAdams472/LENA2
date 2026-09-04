import { gql, useMutation, useQuery } from '@apollo/client';
import { useState } from 'react';
import Link from 'next/link';

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

const CREATE_MEAL_PLAN = gql`
  mutation CreateMealPlan($input: CreateMealPlanInput!) {
    createMealPlan(input: $input) {
      id
      name
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
  const { data, loading, error, refetch } = useQuery(MEAL_PLANS_QUERY);
  const [createMealPlan] = useMutation(CREATE_MEAL_PLAN, {
    onCompleted: () => refetch(),
  });

  const [form, setForm] = useState({
    name: '',
    weekStartDate: '',
    weekStartDayOfWeek: '0',
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createMealPlan({
      variables: {
        input: {
          name: form.name,
          weekStartDate: form.weekStartDate,
          weekStartDayOfWeek: parseInt(form.weekStartDayOfWeek, 10),
        },
      },
    });
    setForm({ name: '', weekStartDate: '', weekStartDayOfWeek: '0' });
  };

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Meal Plans</h1>

      <form onSubmit={handleSubmit} style={{ marginBottom: '2rem' }}>
        <h2>Create meal plan</h2>
        <input
          placeholder="Name"
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          required
        />{' '}
        <input
          type="date"
          value={form.weekStartDate}
          onChange={(e) => setForm({ ...form, weekStartDate: e.target.value })}
          required
        />{' '}
        <input
          type="number"
          min={0}
          max={6}
          placeholder="Week start day (0-6)"
          value={form.weekStartDayOfWeek}
          onChange={(e) => setForm({ ...form, weekStartDayOfWeek: e.target.value })}
        />{' '}
        <button type="submit">Create</button>
      </form>

      <ul>
        {data?.mealPlans?.items.map((plan: MealPlan) => (
          <li key={plan.id}>
            <Link href={`/mealplan/${plan.id}`} legacyBehavior>
              <a>
                {plan.name} — {plan.weekStartDate}{' '}
                {plan.isActive ? '(active)' : '(inactive)'}
              </a>
            </Link>
          </li>
        ))}
      </ul>
    </main>
  );
}
