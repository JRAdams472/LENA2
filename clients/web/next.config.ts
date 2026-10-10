import type { NextConfig } from "next";

// The member app moved under /member for the public marketing site (LEN-121).
// These permanent redirects keep old bookmarks and links working.
const LEGACY_MEMBER_PATHS = [
  "assistant",
  "events",
  "grocery-lists",
  "household",
  "inventory",
  "items",
  "meal-plans",
  "notifications",
  "profile",
  "recipes",
  "users",
  "wine",
];

const nextConfig: NextConfig = {
  output: "standalone",
  async redirects() {
    return LEGACY_MEMBER_PATHS.map((path) => ({
      source: `/${path}/:path*`,
      destination: `/member/${path}/:path*`,
      permanent: true,
    }));
  },
};

export default nextConfig;
