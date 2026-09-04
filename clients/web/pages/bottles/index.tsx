import { gql, useMutation, useQuery } from '@apollo/client';
import { useState } from 'react';
import Link from 'next/link';

const BOTTLES_QUERY = gql`
  query Bottles {
    bottles(page: 1, pageSize: 50) {
      items {
        id
        vineyard
        vintageYear
        bottleSize
      }
      pageInfo {
        totalCount
      }
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

const VINTAGES_QUERY = gql`
  query Vintages {
    vintages {
      id
      year
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

const CREATE_BOTTLE = gql`
  mutation CreateBottle($input: CreateBottleInput!) {
    createBottle(input: $input) {
      id
    }
  }
`;

type Bottle = {
  id: string;
  vineyard?: string | null;
  vintageYear: number;
  bottleSize: string;
};

type Type = { id: string; name: string };
type Country = { id: string; name: string };
type Region = { id: string; name: string };
type Vintage = { id: string; year: number };

export default function Bottles() {
  const { data, loading, error, refetch } = useQuery(BOTTLES_QUERY);
  const { data: typesData } = useQuery(TYPES_QUERY);
  const { data: countriesData } = useQuery(COUNTRIES_QUERY);
  const { data: vintagesData } = useQuery(VINTAGES_QUERY);

  const [form, setForm] = useState({
    typeId: '',
    countryId: '',
    regionId: '',
    vintageYear: '',
    bottleSize: '750ml',
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

  const [createBottle] = useMutation(CREATE_BOTTLE, {
    onCompleted: () => {
      refetch();
      setForm({
        typeId: '',
        countryId: '',
        regionId: '',
        vintageYear: '',
        bottleSize: '750ml',
        vineyard: '',
        abv: '',
        acidity: '',
        tanninLevel: '',
        body: '',
        sweetness: '',
        oakIntegration: false,
      });
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createBottle({
      variables: {
        input: {
          typeId: form.typeId,
          countryId: form.countryId,
          regionId: form.regionId,
          vintageYear: parseInt(form.vintageYear, 10),
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
  const vintages = vintagesData?.vintages ?? [];

  return (
    <main style={{ padding: '2rem' }}>
      <h1>Bottles</h1>

      <form onSubmit={handleSubmit} style={{ marginBottom: '2rem' }}>
        <h2>Create bottle</h2>
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
        <select
          value={form.vintageYear}
          onChange={(e) => setForm({ ...form, vintageYear: e.target.value })}
          required
        >
          <option value="">Vintage</option>
          {vintages.map((v: Vintage) => (
            <option key={v.id} value={v.year}>
              {v.year}
            </option>
          ))}
        </select>{' '}
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
        <button type="submit">Create</button>
      </form>

      <ul>
        {data?.bottles?.items.map((bottle: Bottle) => (
          <li key={bottle.id}>
            <Link href={`/bottles/${bottle.id}`} legacyBehavior>
              <a>
                {bottle.vineyard ?? 'Unknown'} {bottle.vintageYear} — {bottle.bottleSize}
              </a>
            </Link>
          </li>
        ))}
      </ul>
    </main>
  );
}
