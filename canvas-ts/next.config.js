/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  env: {
    NEXT_PUBLIC_SYNC_URL:
      process.env.NEXT_PUBLIC_SYNC_URL || "ws://localhost:8080",
    NEXT_PUBLIC_ASSETS_URL:
      process.env.NEXT_PUBLIC_ASSETS_URL || "http://localhost:8000",
  },
};
module.exports = nextConfig;
