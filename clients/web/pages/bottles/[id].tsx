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
    }
  }
`;

const TYPES_QUERY = gql`
  query Types {
    types {
      id
      name
    }
  }
`;

const COUNTRIES_QUERY = gql`
  query Countries {
    countries {
      id
      name
    }
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

type Type = { id: string; name: string };
type Country = { id: string; name: string };
type Region = { id: string; name: string };

export default function EditBottle() {
  const router = useRouter();
  const id = router.query.id as string | undefined;

  const { data, loading, error } = useQuery(BOTTLE_QUERY, {
    variables: { id },
    skip: !id,
  });
  const { data: typesData } = useQuery(TYPES_QUERY);
  const { data: countriesData } = useQuery(COUNTRIES_QUERY);

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

  const { data: regionsData } = useQuery(REGIONS_QUERY, {
    variables: { countryId: form.countryId },
    skip: !form.countryId,
  });

  const [updateBottle] = useMutation(UPDATE_BOTTLE, {
    onCompleted: () => router.push('/bottles'),
  });

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

  if (loading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  const types = typesData?.types ?? [];
  const countries = countriesData?.countries ?? [];
  const regions = regionsData?.regions ?? [];

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
    </main>
  );
}
