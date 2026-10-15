"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { createVariant, updateVariant } from "@/app/actions/groceries-catalog";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import type { CompareUnit, GroceryStore, VariantRow } from "@/lib/groceries-products";

type EditableVariant = Pick<VariantRow, "variantId" | "brand" | "packLabel" | "packSize" | "brandStoreId">;

function VariantForm({
  productId,
  unit,
  stores,
  variant,
  onDone,
}: {
  productId: string;
  unit: CompareUnit;
  stores: GroceryStore[];
  variant?: EditableVariant;
  onDone: () => void;
}) {
  const t = useTranslations("groceries.products.variantForm");
  const common = useTranslations("common");
  const { state, onSubmit, pending } = useFormAction(variant ? updateVariant : createVariant, () => {
    toast.success(variant ? t("updated") : t("created"));
    onDone();
  });
  const errors = state.fieldErrors ?? {};

  return (
    <form onSubmit={onSubmit} noValidate className="grid gap-4">
      {variant ? <input type="hidden" name="id" value={variant.variantId} /> : <input type="hidden" name="product_id" value={productId} />}
      <Field id="variant-brand" label={t("brand")} hint={t("brandHint")} error={errors.brand}>
        <Input id="variant-brand" name="brand" defaultValue={variant?.brand} maxLength={80} autoComplete="off" aria-invalid={Boolean(errors.brand)} />
      </Field>
      <div className="grid items-start gap-4 sm:grid-cols-2">
        <Field id="variant-label" label={t("packLabel")}>
          <Input id="variant-label" name="pack_label" defaultValue={variant?.packLabel ?? ""} placeholder={t("packLabelPlaceholder")} autoComplete="off" />
        </Field>
        <Field id="variant-size" label={t("packSize", { unit })} hint={t("packSizeHint")} error={errors.pack_size}>
          <Input
            id="variant-size"
            name="pack_size"
            inputMode="decimal"
            defaultValue={variant?.packSize ?? ""}
            autoComplete="off"
            aria-invalid={Boolean(errors.pack_size)}
          />
        </Field>
      </div>
      <Field id="variant-store" label={t("storeBrand")} hint={t("storeBrandHint")}>
        <NativeSelect id="variant-store" name="store_id" defaultValue={variant?.brandStoreId ?? ""}>
          <option value="">{t("storeNone")}</option>
          {stores.map((store) => (
            <option key={store.id} value={store.id}>
              {store.name}
            </option>
          ))}
        </NativeSelect>
      </Field>
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

/** Dialog to add a variant to a product, or edit `variant` when given. */
export function VariantDialog({
  open,
  onOpenChange,
  productId,
  unit,
  stores,
  variant,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  productId: string;
  unit: CompareUnit;
  stores: GroceryStore[];
  variant?: EditableVariant;
}) {
  const t = useTranslations("groceries.products.variantForm");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{variant ? t("editTitle") : t("createTitle")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        {open && <VariantForm productId={productId} unit={unit} stores={stores} variant={variant} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  );
}
