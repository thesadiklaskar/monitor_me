import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  reactStrictMode: true,
  async rewrites() {
    return {
      beforeFiles: [
        { source: "/grafana", destination: "http://grafana:3000/grafana/" },
        { source: "/grafana/:path*", destination: "http://grafana:3000/grafana/:path*" },
      ],
    };
  },
};

export default nextConfig;
