import { getTranslations } from "next-intl/server";

/** Placeholder: the monthly summary arrives with the insights ticket. */
export default async function GroceriesSummaryPage() {
  const t = await getTranslations("groceries");
  return (
    <div className="rounded-3xl bg-card px-6 py-12 text-center text-sm text-muted-foreground ring-1 ring-foreground/5" data-testid="groceries-summary">
      {t("summaryPlaceholder")}
    </div>
  );
}
