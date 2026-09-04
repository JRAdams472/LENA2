import { gql, useMutation, useQuery } from '@apollo/client';
import { useRouter } from 'next/router';
import { useEffect, useState } from 'react';

const BOTTLE_QUERY = gql`
  query Bottle($id: ID!) {
    bottle(id: $id) {
      id
      typeId
      countryId
      regionId
      vineyard
      vintageYear
      bottleSize
      abv
      acidity
      tanninLevel
      body
      sweetness
      oakIntegration
      grapeVarieties {
        grapeVariety { id name }
        percentage
      }
    }
    types { id name }
    countries { id name }
    grapeVarieties { id name }
  }
`;

const REGIONS_QUERY = gql`
  query Regions($countryId: ID!) {
    regions(countryId: $countryId) {
      id
      name
    }
  }
`;

const UPDATE_BOTTLE = gql`
  mutation UpdateBottle($id: ID!, $input: UpdateBottleInput!) {
    updateBottle(id: $id, input: $input) {
      id
    }
  }
`;

const ADD_BOTTLE_GRAPE = gql`
  mutation AddBottleGrapeVariety($input: AddBottleGrapeVarietyInput!) {
    addBottleGrapeVariety(input: $input) {
      grapeVariety { id name }
      percentage
    }
  }
`;

const REMOVE_BOTTLE_GRAPE = gql`
  mutation RemoveBottleGrapeVariety($bottleId: ID!, $grapeVarietyId: ID!) {
    removeBottleGrapeVariety(bottleId: $bottleId, grapeVarietyId: $grapeVarietyId)
  }
`;

type Type = { id: string; name: string };
type Country = { id: string; name: string };
type Region = { id: string; name: string };
type Option = { id: string; name: string };

export default function EditBottle() {
  const router = useRouter();
  const id = router.query.id as string | undefined;
  const { data, loading, error, refetch } = useQuery(BOTTLE_QUERY, {
    variables: { id },
    skip: !id,
  });
  const { data: regionsData } = useQuery(REGIONS_QUERY, {
    variables: { countryId: data?.bottle?.countryId ?? '' },
    skip: !data?.bottle?.countryId,
  });

  const [updateBottle] = useMutation(UPDATE_BOTTLE, {
    onCompleted: () => refetch(),
  });
  const [addBottleGrape] = useMutation(ADD_BOTTLE_GRAPE, {
    onCompleted: () => refetch(),
  });
  const [removeBottleGrape] = useMutation(REMOVE_BOTTLE_GRAPE, {
    onCompleted: () => refetch(),
  });

  const [form, setForm] = useState({
    typeId: '',
    countryId: '',
    regionId: '',
    vintageYear: '',
    bottleSize: '',
    vineyard: '',
    abv: '',
    acidity: '',
    tanninLevel: '',
    body: '',
    sweetness: '',
    oakIntegration: false,
  });
  const [grapeForm, setGrapeForm] = useState({ grapeVarietyId: '', percentage: '' });

  useEffect(() => {
    if (data?.bottle) {
      const b = data.bottle;
      setForm({
        typeId: b.typeId ?? '',
        countryId: b.countryId ?? '',
        regionId: b.regionId ?? '',
        vintageYear: b.vintageYear?.toString() ?? '',
        bottleSize: b.bottleSize ?? '',
        vineyard: b.vineyard ?? '',
        abv: b.abv?.toString() ?? '',
        acidity: b.acidity?.toString() ?? '',
        tanninLevel: b.tanninLevel?.toString() ?? '',
        body: b.body?.toString() ?? '',
        sweetness: b.sweetness?.toString() ?? '',
        oakIntegration: b.oakIntegration ?? false,
      });
    }
  }, [data]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    updateBottle({
      variables: {
        id,
        input: {
          typeId: form.typeId,
          countryId: form.countryId,
          regionId: form.regionId,
          vintageYear: form.vintageYear ? parseInt(form.vintageYear, 10) : null,
          bottleSize: form.bottleSize,
          vineyard: form.vineyard || null,
          abv: form.abv ? parseFloat(form.abv) : null,
          acidity: form.acidity ? parseInt(form.acidity, 10) : null,
          tanninLevel: form.tanninLevel ? parseInt(form.tanninLevel, 10) : null,
          body: form.body ? parseInt(form.body, 10) : null,
          sweetness: form.sweetness ? parseInt(form.sweetness, 10) : null,
          oakIntegration: form.oakIntegration,
        },
      },
    });
  };

  const handleAddGrape = (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !grapeForm.grapeVarietyId) return;
    addBottleGrape({
      variables: {
        input: {
          bottleId: id,
          grapeVarietyId: grapeForm.grapeVarietyId,
          percentage: grapeForm.percentage ? parseInt(grapeForm.percentage, 10) : null,
        },
      },
    });
    setGrapeForm({ grapeVarietyId: '', percentage: '' });
  };

  if (loading || !id) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const types = data?.types ?? [];
  const countries = data?.countries ?? [];
  const regions = regionsData?.regions ?? [];
  const grapeVarieties = data?.grapeVarieties ?? [];
  const bottle = data?.bottle;

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Edit bottle</h1>
      <form onSubmit={handleSubmit}>
        <select
          value={form.typeId}
          onChange={(e) => setForm({ ...form, typeId: e.target.value })}
          required
        >
          <option value="">Type</option>
          {types.map((t: Type) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>{' '}
        <select
          value={form.countryId}
          onChange={(e) => setForm({ ...form, countryId: e.target.value, regionId: '' })}
          required
        >
          <option value="">Country</option>
          {countries.map((c: Country) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>{' '}
        <select
          value={form.regionId}
          onChange={(e) => setForm({ ...form, regionId: e.target.value })}
          required
          disabled={!form.countryId}
        >
          <option value="">Region</option>
          {regions.map((r: Region) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </select>{' '}
        <input
          placeholder="Vintage year"
          value={form.vintageYear}
          onChange={(e) => setForm({ ...form, vintageYear: e.target.value })}
          required
        />{' '}
        <input
          placeholder="Bottle size"
          value={form.bottleSize}
          onChange={(e) => setForm({ ...form, bottleSize: e.target.value })}
          required
        />{' '}
        <input
          placeholder="Vineyard"
          value={form.vineyard}
          onChange={(e) => setForm({ ...form, vineyard: e.target.value })}
        />{' '}
        <input
          placeholder="ABV"
          value={form.abv}
          onChange={(e) => setForm({ ...form, abv: e.target.value })}
        />{' '}
        <input
          placeholder="Acidity"
          value={form.acidity}
          onChange={(e) => setForm({ ...form, acidity: e.target.value })}
        />{' '}
        <input
          placeholder="Tannin"
          value={form.tanninLevel}
          onChange={(e) => setForm({ ...form, tanninLevel: e.target.value })}
        />{' '}
        <input
          placeholder="Body"
          value={form.body}
          onChange={(e) => setForm({ ...form, body: e.target.value })}
        />{' '}
        <input
          placeholder="Sweetness"
          value={form.sweetness}
          onChange={(e) => setForm({ ...form, sweetness: e.target.value })}
        />{' '}
        <label>
          <input
            type="checkbox"
            checked={form.oakIntegration}
            onChange={(e) => setForm({ ...form, oakIntegration: e.target.checked })}
          />{' '}
          Oak integration
        </label>{' '}
        <button type="submit">Update</button>
      </form>

      <h2 style={{ marginTop: '2rem' }}>Grape varieties</h2>
      <ul>
        {(bottle?.grapeVarieties ?? []).map((g: any) => (
          <li key={g.grapeVariety.id}>
            {g.grapeVariety.name}
            {g.percentage ? ` (${g.percentage}%)` : ''}{' '}
            <button
              onClick={() =>
                removeBottleGrape({
                  variables: { bottleId: id, grapeVarietyId: g.grapeVariety.id },
                })
              }
            >
              Remove
            </button>
          </li>
        ))}
      </ul>
      <form onSubmit={handleAddGrape}>
        <select
          value={grapeForm.grapeVarietyId}
          onChange={(e) => setGrapeForm({ ...grapeForm, grapeVarietyId: e.target.value })}
          required
        >
          <option value="">Grape variety</option>
          {grapeVarieties.map((g: Option) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </select>{' '}
        <input
          placeholder="%"
          value={grapeForm.percentage}
          onChange={(e) => setGrapeForm({ ...grapeForm, percentage: e.target.value })}
        />{' '}
        <button type="submit">Add</button>
      </form>
    </main>
  );
}
