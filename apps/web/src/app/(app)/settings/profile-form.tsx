"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { updateProfile } from "@/app/actions/profile";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
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
    <form onSubmit={onSubmit} className="grid max-w-md gap-4">
      <Field id="email" label={t("settings.email")}>
        <Input id="email" value={email} disabled readOnly />
      </Field>
      <Field id="name" label={t("settings.name")} error={state.fieldErrors?.name}>
        <Input id="name" name="name" defaultValue={name} maxLength={100} autoComplete="name" />
      </Field>
      <Field id="locale" label={t("settings.language")}>
        <NativeSelect id="locale" name="locale" defaultValue={locale}>
          {locales.map((l) => (
            <option key={l} value={l}>
              {t(`settings.languages.${l}`)}
            </option>
          ))}
        </NativeSelect>
      </Field>
      {state.message && !state.ok && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <div>
        <Button type="submit" disabled={pending}>
          {pending ? t("common.saving") : t("common.save")}
        </Button>
      </div>
    </form>
  );
}
