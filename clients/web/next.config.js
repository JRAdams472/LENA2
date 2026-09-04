/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: '/graphql',
        destination: process.env.NEXT_PUBLIC_LENA_API_URL || 'http://localhost:8080/graphql',
      },
    ];
  },
};

module.exports = nextConfig;
