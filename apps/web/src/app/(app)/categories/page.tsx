import { PlusIcon } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { CategoryDialog } from "@/components/categories/category-dialog";
import { CategoryList } from "@/components/categories/category-list";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { authedApi, expectData } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("categories");
  return { title: t("title") };
}

export default async function CategoriesPage({ searchParams }: { searchParams: Promise<{ archived?: string }> }) {
  const t = await getTranslations();
  const showArchived = (await searchParams).archived === "1";
  const api = await authedApi();
  const categories = expectData(
    await api.GET("/api/v1/categories", { params: { query: { include_archived: showArchived } } }),
  ).items;

  return (
    <>
      <PageHeader
        title={t("categories.title")}
        actions={
          <Button nativeButton={false} render={<Link href={showArchived ? "/categories" : "/categories?archived=1"} />} variant="ghost" size="sm">
            {t(showArchived ? "accounts.hideArchived" : "accounts.showArchived")}
          </Button>
        }
      />
      <div className="grid gap-4 md:grid-cols-2">
        {(["expense", "income"] as const).map((kind) => (
          <Card key={kind}>
            <CardHeader>
              <CardTitle>{t(kind === "expense" ? "categories.expenseCategories" : "categories.incomeCategories")}</CardTitle>
              <CardAction>
                <CategoryDialog
                  defaultKind={kind}
                  trigger={
                    <Button variant="outline" size="sm">
                      <PlusIcon />
                      {t("categories.add")}
                    </Button>
                  }
                />
              </CardAction>
            </CardHeader>
            <CardContent>
              <CategoryList categories={categories.filter((c) => c.kind === kind)} />
            </CardContent>
          </Card>
        ))}
      </div>
    </>
  );
}
