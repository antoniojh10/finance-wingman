"use client";

import { ArchiveIcon, ArchiveRestoreIcon, PencilIcon, Trash2Icon } from "lucide-react";
import { useTranslations } from "next-intl";
import { useTransition } from "react";
import { toast } from "sonner";

import { deleteCategory, setCategoryArchived } from "@/app/actions/categories";
import { ConfirmAction } from "@/components/confirm-action";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

import { CategoryDialog, type EditableCategory } from "./category-dialog";

export type CategoryListItem = EditableCategory & { archived: boolean };

export function CategoryList({ categories }: { categories: CategoryListItem[] }) {
  const t = useTranslations();
  const [pending, startTransition] = useTransition();

  function toggleArchived(category: CategoryListItem) {
    startTransition(async () => {
      const result = await setCategoryArchived(category.id, !category.archived);
      if (result.ok) {
        toast.success(t(category.archived ? "categories.restoredToast" : "categories.archivedToast"));
      } else {
        toast.error(result.message);
      }
    });
  }

  if (categories.length === 0) {
    return <p className="py-4 text-sm text-muted-foreground">{t("categories.empty")}</p>;
  }

  return (
    <ul className="divide-y">
      {categories.map((category) => (
        <li key={category.id} className="flex items-center gap-3 py-2.5" data-testid="category-row">
          <span
            aria-hidden
            className="size-3 shrink-0 rounded-full border border-foreground/10"
            style={{ backgroundColor: category.color ?? "var(--muted-foreground)" }}
          />
          <p className="flex min-w-0 flex-1 items-center gap-2 truncate text-sm">
            {category.name}
            {category.archived && <Badge variant="secondary">{t("common.archived")}</Badge>}
          </p>
          <div className="flex shrink-0 gap-0.5">
            <CategoryDialog
              category={category}
              trigger={
                <Button variant="ghost" size="icon-sm" aria-label={t("common.edit")}>
                  <PencilIcon />
                </Button>
              }
            />
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t(category.archived ? "common.restore" : "common.archive")}
              title={t(category.archived ? "common.restore" : "common.archive")}
              onClick={() => toggleArchived(category)}
              disabled={pending}
            >
              {category.archived ? <ArchiveRestoreIcon /> : <ArchiveIcon />}
            </Button>
            <ConfirmAction
              trigger={
                <Button variant="ghost" size="icon-sm" aria-label={t("common.delete")}>
                  <Trash2Icon />
                </Button>
              }
              title={t("categories.deleteTitle")}
              description={t("categories.deleteDescription")}
              confirmLabel={t("common.delete")}
              successMessage={t("categories.deleted")}
              action={() => deleteCategory(category.id)}
            />
          </div>
        </li>
      ))}
    </ul>
  );
}
