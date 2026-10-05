"use client";

import { useTranslations } from "next-intl";
import { useTheme } from "next-themes";
import { useSyncExternalStore } from "react";

import { ChipRadioGroup } from "@/components/chip-radio";

const THEMES = ["system", "light", "dark"] as const;

const subscribe = () => () => {};

/** Lets the user force light or dark mode, or follow the OS; stored per browser by next-themes. */
export function ThemePicker() {
  const t = useTranslations("settings");
  const { theme, setTheme } = useTheme();
  // The stored theme is unknown on the server: select nothing until mounted.
  const mounted = useSyncExternalStore(subscribe, () => true, () => false);

  return (
    <ChipRadioGroup
      name="theme"
      label={t("theme")}
      options={THEMES.map((value) => ({ value, label: t(`themes.${value}`) }))}
      value={mounted ? (theme ?? "system") : ""}
      onChange={setTheme}
    />
  );
}
