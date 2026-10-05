"use client";

import { ArchiveIcon, ArchiveRestoreIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState, useTransition, type CSSProperties } from "react";
import { toast } from "sonner";

import { deleteCategory, setCategoryArchived } from "@/app/actions/categories";
import { ColorTile } from "@/components/color-tile";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/page-header";
import { RowActions } from "@/components/row-actions";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import { CategoryDialog, type EditableCategory } from "./category-dialog";

export type CategoryListItem = EditableCategory & { archived: boolean };

/** What was spent or earned in a category this month, per currency. */
export type CategoryMonthTotal = { currency: string; minor_units: number; total: number };

type Kind = "expense" | "income";

/** Categories page body: expense/income tabs over a grid of category cards. */
export function CategoryBoard({
  categories,
  totals,
  showArchived,
}: {
  categories: CategoryListItem[];
  totals: Record<string, CategoryMonthTotal[]>;
  showArchived: boolean;
}) {
  const t = useTranslations();
  const [kind, setKind] = useState<Kind>("expense");
  const visible = categories.filter((c) => c.kind === kind);

  return (
    <>
      <PageHeader
        title={t("categories.title")}
        actions={
          <CategoryDialog
            defaultKind={kind}
            trigger={
              <Button>
                <PlusIcon />
                {t("categories.add")}
              </Button>
            }
          />
        }
      />
      <div className="grid grid-cols-1 gap-4">
        <div role="tablist" aria-label={t("categories.kind")} className="grid grid-cols-2 gap-1 rounded-2xl bg-muted p-1 sm:w-80">
          {(["expense", "income"] as const).map((option) => (
            <button
              key={option}
              type="button"
              role="tab"
              id={`tab-${option}`}
              aria-selected={kind === option}
              aria-controls="category-panel"
              onClick={() => setKind(option)}
              className={cn(
                "min-h-11 rounded-xl text-sm font-bold transition-colors",
                kind === option ? "bg-card shadow-sm" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {t(`categories.kinds.${option}`)}
            </button>
          ))}
        </div>
        <section id="category-panel" role="tabpanel" aria-labelledby={`tab-${kind}`} className="grid grid-cols-1 gap-3">
          <h2 className="px-1 font-sans text-xs font-bold tracking-wider text-muted-foreground uppercase">
            {t(kind === "expense" ? "categories.expenseCategories" : "categories.incomeCategories")}
          </h2>
          <CategoryList categories={visible} totals={totals} />
        </section>
        <Link
          href={showArchived ? "/categories" : "/categories?archived=1"}
          className="flex min-h-11 items-center justify-self-center px-4 text-sm font-bold underline underline-offset-4"
        >
          {t(showArchived ? "accounts.hideArchived" : "accounts.showArchived")}
        </Link>
      </div>
    </>
  );
}

export function CategoryList({
  categories,
  totals,
}: {
  categories: CategoryListItem[];
  totals: Record<string, CategoryMonthTotal[]>;
}) {
  const t = useTranslations();
  if (categories.length === 0) {
    return <p className="rounded-3xl bg-card py-10 text-center text-sm text-muted-foreground">{t("categories.empty")}</p>;
  }
  return (
    <ul className="grid grid-cols-2 gap-2.5 md:grid-cols-3 xl:grid-cols-4">
      {categories.map((category) => (
        <CategoryCard key={category.id} category={category} totals={totals[category.id] ?? []} />
      ))}
    </ul>
  );
}

function CategoryCard({ category, totals }: { category: CategoryListItem; totals: CategoryMonthTotal[] }) {
  const t = useTranslations();
  const [pending, startTransition] = useTransition();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);

  function toggleArchived() {
    startTransition(async () => {
      const result = await setCategoryArchived(category.id, !category.archived);
      if (result.ok) {
        toast.success(t(category.archived ? "categories.restoredToast" : "categories.archivedToast"));
      } else {
        toast.error(result.message);
      }
    });
  }

  return (
    <li
      className={cn("relative grid min-w-0 grid-cols-1 gap-3.5 overflow-hidden rounded-[22px] bg-card p-3.5 ring-1 ring-foreground/5", category.archived && "opacity-70")}
      data-testid="category-row"
    >
      <span
        aria-hidden
        className="absolute -right-4.5 -bottom-4.5 size-16 rounded-full bg-(--tile) opacity-90"
        style={{ "--tile": category.color ?? "var(--muted-foreground)" } as CSSProperties}
      />
      <div className="flex items-start justify-between">
        <ColorTile color={category.color} label={category.name} className="size-10 rounded-[13px] text-[17px]" />
        <RowActions
          className="-mt-1.5 -mr-1.5 text-muted-foreground"
          actions={[
            { label: t("common.edit"), icon: PencilIcon, onSelect: () => setEditing(true) },
            {
              label: t(category.archived ? "common.restore" : "common.archive"),
              icon: category.archived ? ArchiveRestoreIcon : ArchiveIcon,
              onSelect: toggleArchived,
              disabled: pending,
            },
            { label: t("common.delete"), icon: Trash2Icon, onSelect: () => setDeleting(true), destructive: true },
          ]}
        />
      </div>
      <div className="relative grid gap-0.5">
        <p className="truncate text-[15.5px] font-bold">{category.name}</p>
        {category.archived && <Badge variant="secondary">{t("common.archived")}</Badge>}
        <p className="text-[12.5px] text-muted-foreground">{t("categories.thisMonth")}</p>
        {totals.length === 0 ? (
          <p className="font-heading text-lg font-bold text-muted-foreground">—</p>
        ) : (
          totals.map((total) => (
            <Money
              key={total.currency}
              amount={total.total}
              currency={total.currency}
              minorUnits={total.minor_units}
              className="font-heading text-lg font-bold"
            />
          ))
        )}
      </div>
      <CategoryDialog category={category} open={editing} onOpenChange={setEditing} />
      <ConfirmAction
        open={deleting}
        onOpenChange={setDeleting}
        title={t("categories.deleteTitle")}
        description={t("categories.deleteDescription")}
        confirmLabel={t("common.delete")}
        successMessage={t("categories.deleted")}
        action={() => deleteCategory(category.id)}
      />
    </li>
  );
}
