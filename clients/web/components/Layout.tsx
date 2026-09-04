import type { ReactNode } from 'react';
import Link from 'next/link';

export function Layout({ children }: { children: ReactNode }) {
  return (
    <div>
      <nav style={{ padding: '1rem', borderBottom: '1px solid #ccc' }}>
        <Link href="/" legacyBehavior>
          <a>Home</a>
        </Link>
        {' | '}
        <Link href="/items" legacyBehavior>
          <a>Items</a>
        </Link>
        {' | '}
        <Link href="/recipes" legacyBehavior>
          <a>Recipes</a>
        </Link>
        {' | '}
        <Link href="/mealplans" legacyBehavior>
          <a>Meal Plans</a>
        </Link>
        {' | '}
        <Link href="/wine" legacyBehavior>
          <a>Wine</a>
        </Link>
        {' | '}
        <Link href="/bottles" legacyBehavior>
          <a>Bottles</a>
        </Link>
        {' | '}
        <Link href="/grocery" legacyBehavior>
          <a>Grocery</a>
        </Link>
      </nav>
      {children}
    </div>
  );
}
