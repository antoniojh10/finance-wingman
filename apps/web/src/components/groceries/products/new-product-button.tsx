"use client";

import { PlusIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import type { ProductGroup } from "@/lib/groceries-products";

import { ProductDialog } from "./product-dialog";

/** The "New product" button with its dialog. */
export function NewProductButton({ groups }: { groups: ProductGroup[] }) {
  const t = useTranslations("groceries.products");
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button size="lg" onClick={() => setOpen(true)}>
        <PlusIcon />
        {t("new")}
      </Button>
      <ProductDialog open={open} onOpenChange={setOpen} groups={groups} />
    </>
  );
}
