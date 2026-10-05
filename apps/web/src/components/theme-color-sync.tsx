"use client";

import { useTheme } from "next-themes";
import { useEffect } from "react";

// Keep in sync with `viewport.themeColor` in app/layout.tsx.
const THEME_COLORS = { light: "#f4f5fb", dark: "#0e0c22" } as const;

/**
 * The layout's theme-color metas follow the OS through media queries; when the
 * user forces a theme, point every one of them at that theme's color instead.
 */
export function ThemeColorSync() {
  const { resolvedTheme } = useTheme();

  useEffect(() => {
    if (resolvedTheme !== "light" && resolvedTheme !== "dark") return;
    document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]').forEach((meta) => {
      meta.content = THEME_COLORS[resolvedTheme];
    });
  }, [resolvedTheme]);

  return null;
}
