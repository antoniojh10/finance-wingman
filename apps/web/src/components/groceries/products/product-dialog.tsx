"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { createProduct, updateProduct } from "@/app/actions/groceries-catalog";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { COMPARE_UNITS, type CatalogProduct, type ProductGroup } from "@/lib/groceries-products";

function ProductForm({ product, groups, onDone }: { product?: CatalogProduct; groups: ProductGroup[]; onDone: () => void }) {
  const t = useTranslations("groceries.products.form");
  const common = useTranslations("common");
  const { state, onSubmit, pending } = useFormAction(product ? updateProduct : createProduct, () => {
    toast.success(product ? t("updated") : t("created"));
    onDone();
  });
  const errors = state.fieldErrors ?? {};

  return (
    <form onSubmit={onSubmit} noValidate className="grid gap-4">
      {product && <input type="hidden" name="id" value={product.id} />}
      <Field id="product-name" label={t("name")} error={errors.name}>
        <Input id="product-name" name="name" defaultValue={product?.name} maxLength={120} autoComplete="off" placeholder={t("namePlaceholder")} aria-invalid={Boolean(errors.name)} />
      </Field>
      <Field id="product-group" label={t("group")}>
        <NativeSelect id="product-group" name="group_id" defaultValue={product?.group_id ?? ""}>
          <option value="">{t("groupNone")}</option>
          {groups.map((group) => (
            <option key={group.id} value={group.id}>
              {group.name}
            </option>
          ))}
        </NativeSelect>
      </Field>
      <div className="grid items-start gap-4 sm:grid-cols-2">
        <Field id="product-unit" label={t("compareUnit")} error={errors.compare_unit}>
          <NativeSelect id="product-unit" name="compare_unit" defaultValue={product?.compare_unit ?? "kg"}>
            {COMPARE_UNITS.map((unit) => (
              <option key={unit} value={unit}>
                {t(`unit.${unit}`)}
              </option>
            ))}
          </NativeSelect>
        </Field>
        <Field id="product-multiplier" label={t("multiplier")} hint={t("multiplierHint")} error={errors.display_multiplier}>
          <Input
            id="product-multiplier"
            name="display_multiplier"
            inputMode="numeric"
            defaultValue={product?.display_multiplier ?? 1}
            autoComplete="off"
            aria-invalid={Boolean(errors.display_multiplier)}
          />
        </Field>
      </div>
      {state.message && !state.ok && !state.fieldErrors && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <Button type="submit" size="lg" disabled={pending}>
        {pending ? common("saving") : common("save")}
      </Button>
    </form>
  );
}

/** Dialog to create a product, or edit `product` when given. */
export function ProductDialog({
  open,
  onOpenChange,
  product,
  groups,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  product?: CatalogProduct;
  groups: ProductGroup[];
}) {
  const t = useTranslations("groceries.products.form");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{product ? t("editTitle") : t("createTitle")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        {open && <ProductForm product={product} groups={groups} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  );
}
