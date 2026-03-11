/** @type {import('next').NextConfig} */
const nextConfig = {
  transpilePackages: ["@1tok/contracts"],
  output: "standalone",
};

export default nextConfig;
