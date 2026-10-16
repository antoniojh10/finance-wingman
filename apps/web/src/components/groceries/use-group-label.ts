import { useTranslations } from "next-intl";

import { groupLabel, type GroupNameTranslator } from "@/lib/groceries-products";

/** Returns a function that names a product group in the current language (see groupLabel). */
export function useGroupLabel() {
  const t = useTranslations("groceries.groupNames") as unknown as GroupNameTranslator;
  return (group: { name: string; default_key?: string | null }) => groupLabel(group, t);
}
