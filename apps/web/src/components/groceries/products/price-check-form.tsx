"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { createPriceCheck } from "@/app/actions/groceries-catalog";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";

export type PriceCheckVariant = { id: string; label: string };

/**
 * Records the pack price seen at a store without buying anything. Only
 * variants with a pack size can be checked: the unit price comes from it.
 */
export function PriceCheckForm({
  variants,
  stores,
  currencies,
  today,
}: {
  variants: PriceCheckVariant[];
  stores: { id: string; name: string }[];
  currencies: string[];
  today: string;
}) {
  const t = useTranslations("groceries.products.priceCheck");
  const [formKey, setFormKey] = useState(0);
  const { state, onSubmit, pending } = useFormAction(createPriceCheck, () => {
    toast.success(t("saved"));
    setFormKey((key) => key + 1);
  });
  const errors = state.fieldErrors ?? {};

  if (variants.length === 0 || stores.length === 0) {
    return <p className="text-sm text-muted-foreground">{variants.length === 0 ? t("noVariants") : t("noStores")}</p>;
  }

  return (
    <form key={formKey} onSubmit={onSubmit} noValidate className="grid gap-4" aria-label={t("heading")}>
      <div className="grid items-start gap-4 sm:grid-cols-2">
        <Field id="check-variant" label={t("variant")} error={errors.variant_id}>
          <NativeSelect id="check-variant" name="variant_id" defaultValue={variants.length === 1 ? variants[0].id : ""} aria-invalid={Boolean(errors.variant_id)}>
            <option value="">{t("variantPlaceholder")}</option>
            {variants.map((variant) => (
              <option key={variant.id} value={variant.id}>
                {variant.label}
              </option>
            ))}
          </NativeSelect>
        </Field>
        <Field id="check-store" label={t("store")} error={errors.store_id}>
          <NativeSelect id="check-store" name="store_id" defaultValue={stores.length === 1 ? stores[0].id : ""} aria-invalid={Boolean(errors.store_id)}>
            <option value="">{t("storePlaceholder")}</option>
            {stores.map((store) => (
              <option key={store.id} value={store.id}>
                {store.name}
              </option>
            ))}
          </NativeSelect>
        </Field>
      </div>
      <div className="grid items-start gap-4 sm:grid-cols-[1fr_7rem_1fr]">
        <Field id="check-price" label={t("price")} hint={t("priceHint")} error={errors.price}>
          <Input id="check-price" name="price" inputMode="decimal" autoComplete="off" placeholder="0.00" aria-invalid={Boolean(errors.price)} />
        </Field>
        <Field id="check-currency" label={t("currency")} error={errors.currency}>
          <NativeSelect id="check-currency" name="currency" defaultValue={currencies[0]}>
            {currencies.map((currency) => (
              <option key={currency} value={currency}>
                {currency}
              </option>
            ))}
          </NativeSelect>
        </Field>
        <Field id="check-date" label={t("date")}>
          <Input id="check-date" name="observed_on" type="date" defaultValue={today} />
        </Field>
      </div>
      <Field id="check-note" label={t("note")}>
        <Input id="check-note" name="note" maxLength={200} autoComplete="off" placeholder={t("notePlaceholder")} />
      </Field>
      {state.message && !state.ok && !state.fieldErrors && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <div className="flex sm:justify-end">
        <Button type="submit" size="lg" disabled={pending} className="max-sm:w-full">
          {pending ? t("saving") : t("save")}
        </Button>
      </div>
    </form>
  );
}
