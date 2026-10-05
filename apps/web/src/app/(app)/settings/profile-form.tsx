"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { updateProfile } from "@/app/actions/profile";
import { SegmentedRadioGroup } from "@/components/chip-radio";
import { Field } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { locales } from "@/i18n/locales";

export function ProfileForm({ name, email, locale }: { name: string; email: string; locale: string }) {
  const t = useTranslations();
  const router = useRouter();
  const { state, onSubmit, pending } = useFormAction(updateProfile, () => {
    toast.success(t("settings.saved"));
    // Re-render server components with the new language.
    router.refresh();
  });

  return (
    <form onSubmit={onSubmit} className="grid gap-4">
      <Field id="name" label={t("settings.name")} error={state.fieldErrors?.name}>
        <Input id="name" name="name" defaultValue={name} maxLength={100} autoComplete="name" className="bg-background" />
      </Field>
      <Field id="email" label={t("settings.email")}>
        <Input id="email" value={email} disabled readOnly className="bg-background" />
      </Field>
      <SegmentedRadioGroup
        name="locale"
        label={t("settings.language")}
        options={locales.map((l) => ({ value: l, label: t(`settings.languages.${l}`) }))}
        defaultValue={locale}
      />
      {state.message && !state.ok && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <Button type="submit" size="lg" disabled={pending}>
        {pending ? t("common.saving") : t("common.save")}
      </Button>
    </form>
  );
}
