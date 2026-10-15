import { useTranslations } from "next-intl";

import type { CompareUnit } from "@/lib/groceries-products";

/**
 * Returns a function describing how a product is priced: "per kg", "per
 * litre", "per unit" or, with a display multiplier, "per dozen" / "per 6 units".
 */
export function usePriceUnit() {
  const t = useTranslations("groceries.products.unit");
  return (unit: CompareUnit, multiplier = 1): string => {
    if (multiplier === 12 && unit === "unit") {
      return t("perDozen");
    }
    if (multiplier > 1) {
      return t("perMany", { count: multiplier, unit });
    }
    return t("per", { unit });
  };
}
