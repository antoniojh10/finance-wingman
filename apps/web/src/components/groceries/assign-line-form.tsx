"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";

import { assignReceiptLine } from "@/app/actions/groceries";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import type { CatalogEntry } from "@/lib/groceries";
import { cn } from "@/lib/utils";

export type GroupOption = { id: string; name: string };

/**
 * Maps one receipt line to a product variant: pick an existing product and
 * variant, or describe a new one. The API remembers the store's wording, so
 * the next receipt with the same text is assigned automatically.
 */
export function AssignLineForm({
  receiptId,
  lineId,
  catalog,
  groups,
  onDone,
  onCancel,
}: {
  receiptId: string;
  lineId: string;
  catalog: CatalogEntry[];
  groups: GroupOption[];
  onDone: () => void;
  onCancel: () => void;
}) {
  const t = useTranslations("groceries.assign");
  const [mode, setMode] = useState<"existing" | "new">(catalog.length > 0 ? "existing" : "new");
  const [productId, setProductId] = useState("");
  const { state, onSubmit, pending } = useFormAction(assignReceiptLine, onDone);
  const errors = state.fieldErrors ?? {};
  const variants = catalog.find((entry) => entry.id === productId)?.variants ?? [];
  const id = `assign-${lineId}`;

  return (
    <form onSubmit={onSubmit} noValidate className="grid min-w-0 gap-4 rounded-2xl border border-border bg-muted/40 p-4">
      <input type="hidden" name="receipt_id" value={receiptId} />
      <input type="hidden" name="line_id" value={lineId} />
      <input type="hidden" name="mode" value={mode} />

      <div className="grid grid-cols-2 gap-1 rounded-2xl bg-muted p-1" role="group">
        {(["existing", "new"] as const).map((value) => (
          <button
            key={value}
            type="button"
            aria-pressed={mode === value}
            disabled={value === "existing" && catalog.length === 0}
            onClick={() => setMode(value)}
            className={cn(
              "min-h-11 rounded-xl px-3 text-sm font-bold text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-50",
              mode === value && "bg-card text-foreground shadow-sm",
            )}
          >
            {value === "existing" ? t("modeExisting") : t("modeNew")}
          </button>
        ))}
      </div>

      {mode === "existing" ? (
        <>
          <Field id={`${id}-product`} label={t("product")}>
            <NativeSelect
              id={`${id}-product`}
              value={productId}
              onChange={(event) => setProductId(event.target.value)}
            >
              <option value="">{t("productPlaceholder")}</option>
              {catalog.map((entry) => (
                <option key={entry.id} value={entry.id}>
                  {entry.name}
                </option>
              ))}
            </NativeSelect>
          </Field>
          <Field id={`${id}-variant`} label={t("variant")} error={errors.variant_id}>
            <NativeSelect id={`${id}-variant`} name="variant_id" defaultValue="" disabled={!productId} aria-invalid={Boolean(errors.variant_id)} key={productId}>
              <option value="">{t("variantPlaceholder")}</option>
              {variants.map((variant) => (
                <option key={variant.id} value={variant.id}>
                  {variant.label}
                </option>
              ))}
            </NativeSelect>
          </Field>
        </>
      ) : (
        <div className="grid items-start gap-4 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <Field id={`${id}-name`} label={t("newProduct")} hint={t("newProductHint")} error={errors.product}>
              <Input id={`${id}-name`} name="product" list={`${id}-products`} autoComplete="off" aria-invalid={Boolean(errors.product)} />
              <datalist id={`${id}-products`}>
                {catalog.map((entry) => (
                  <option key={entry.id} value={entry.name} />
                ))}
              </datalist>
            </Field>
          </div>
          <Field id={`${id}-brand`} label={t("brand")} hint={t("brandHint")}>
            <Input id={`${id}-brand`} name="brand" autoComplete="off" />
          </Field>
          <Field id={`${id}-label`} label={t("packLabel")}>
            <Input id={`${id}-label`} name="pack_label" placeholder={t("packLabelPlaceholder")} autoComplete="off" />
          </Field>
          <Field id={`${id}-size`} label={t("packSize")} hint={t("packSizeHint")} error={errors.pack_size}>
            <Input id={`${id}-size`} name="pack_size" inputMode="decimal" autoComplete="off" aria-invalid={Boolean(errors.pack_size)} />
          </Field>
          <Field id={`${id}-unit`} label={t("compareUnit")} error={errors.compare_unit}>
            <NativeSelect id={`${id}-unit`} name="compare_unit" defaultValue="unit">
              <option value="kg">{t("unitKg")}</option>
              <option value="l">{t("unitL")}</option>
              <option value="unit">{t("unitUnit")}</option>
            </NativeSelect>
          </Field>
          <div className="sm:col-span-2">
            <Field id={`${id}-group`} label={t("group")}>
              <NativeSelect id={`${id}-group`} name="group" defaultValue="">
                <option value="">{t("groupNone")}</option>
                {groups.map((group) => (
                  <option key={group.id} value={group.name}>
                    {group.name}
                  </option>
                ))}
              </NativeSelect>
            </Field>
          </div>
        </div>
      )}

      {state.message && !state.ok && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button type="button" variant="ghost" size="lg" onClick={onCancel}>
          {t("cancel")}
        </Button>
        <Button type="submit" size="lg" disabled={pending}>
          {pending ? t("saving") : t("save")}
        </Button>
      </div>
    </form>
  );
}
