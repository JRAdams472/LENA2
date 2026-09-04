import { gql, useMutation, useQuery } from '@apollo/client';
import { useRouter } from 'next/router';
import { useEffect, useState } from 'react';

const MEAL_PLAN_QUERY = gql`
  query MealPlan($id: ID!) {
    mealPlan(id: $id) {
      id
      name
      weekStartDate
      isActive
    }
  }
`;

const UPDATE_MEAL_PLAN = gql`
  mutation UpdateMealPlan($id: ID!, $input: CreateMealPlanInput!) {
    updateMealPlan(id: $id, input: $input) {
      id
      name
    }
  }
`;

export default function EditMealPlan() {
  const router = useRouter();
  const id = router.query.id as string | undefined;

  const { data, loading, error } = useQuery(MEAL_PLAN_QUERY, {
    variables: { id },
    skip: !id,
  });
  const [updateMealPlan] = useMutation(UPDATE_MEAL_PLAN, {
    onCompleted: () => router.push('/mealplans'),
  });

  const [form, setForm] = useState({
    name: '',
    weekStartDate: '',
    weekStartDayOfWeek: '0',
  });

  useEffect(() => {
    if (data?.mealPlan) {
      const p = data.mealPlan;
      setForm({
        name: p.name ?? '',
        weekStartDate: p.weekStartDate ?? '',
        weekStartDayOfWeek: p.weekStartDayOfWeek?.toString() ?? '0',
      });
    }
  }, [data]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    updateMealPlan({
      variables: {
        id,
        input: {
          name: form.name,
          weekStartDate: form.weekStartDate,
          weekStartDayOfWeek: parseInt(form.weekStartDayOfWeek, 10),
        },
      },
    });
  };

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Edit meal plan</h1>
      <form onSubmit={handleSubmit}>
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
        <button type="submit">Update</button>
      </form>
    </main>
  );
}
