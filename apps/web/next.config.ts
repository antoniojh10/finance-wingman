import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin();

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  // The floating dev indicator covers the mobile bottom navigation.
  devIndicators: false,
};

export default withNextIntl(nextConfig);
