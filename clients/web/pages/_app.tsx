import { ApolloProvider } from '@apollo/client';
import { apolloClient } from '../lib/apolloClient';
import { Layout } from '../components/Layout';
import type { AppProps } from 'next/app';

export default function App({ Component, pageProps }: AppProps) {
  return (
    <ApolloProvider client={apolloClient}>
      <Layout>
        <Component {...pageProps} />
      </Layout>
    </ApolloProvider>
  );
}
