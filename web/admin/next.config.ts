import type { NextConfig } from "next";
const config: NextConfig = {
  outputFileTracingRoot: process.cwd(),
  ...(process.env.NODE_ENV === "production"
    ? { output: "export" as const }
    : {
        async rewrites() {
          return {
            beforeFiles: [
              {
                source: "/api/v1/:path*",
                destination: `${process.env.AXOTTLE_DEV_API_URL || "http://127.0.0.1:8080"}/api/v1/:path*`,
              },
            ],
          };
        },
      }),
  images: { unoptimized: true },
  productionBrowserSourceMaps: false,
  generateBuildId: async () =>
    process.env.AXOTTLE_UI_BUILD_ID || "axottle-core",
};
export default config;
