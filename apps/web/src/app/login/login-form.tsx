"use client";

import { MailIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import { useActionState, useState } from "react";

import { requestLogin, verifyCode, type LoginState } from "@/app/actions/auth";
import { Field } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

/**
 * Two-step sign-in: request an email, then enter the 6-digit code (or click
 * the link in the email).
 */
export function LoginForm() {
  const t = useTranslations("auth");
  const [requestState, requestAction, requesting] = useActionState(requestLogin, { step: "email" } as LoginState);
  const [verifyState, verifyAction, verifying] = useActionState(verifyCode, { step: "code" } as LoginState);
  const [editingEmail, setEditingEmail] = useState(false);

  const showCode = requestState.step === "code" && !editingEmail;

  if (!showCode) {
    return (
      <form action={(data) => { setEditingEmail(false); requestAction(data); }} className="grid gap-4">
        <Field id="email" label={t("email")} error={requestState.error}>
          <Input
            id="email"
            name="email"
            type="email"
            autoComplete="email"
            placeholder={t("emailPlaceholder")}
            required
            autoFocus
            defaultValue={requestState.email}
            aria-invalid={Boolean(requestState.error)}
            className="h-13.5 rounded-2xl px-4 text-base"
          />
        </Field>
        <Button type="submit" size="lg" disabled={requesting} className="h-14 rounded-[18px]">
          <MailIcon />
          {requesting ? t("sending") : t("sendLink")}
        </Button>
      </form>
    );
  }

  return (
    <div className="grid gap-4 rounded-3xl bg-card p-5 ring-1 ring-foreground/5">
      <div className="grid gap-1.5">
        <h2 className="text-2xl font-bold tracking-tight">{t("checkEmail")}</h2>
        <p className="text-sm leading-relaxed text-muted-foreground">{t("sentTo", { email: requestState.email ?? "" })}</p>
      </div>
      <form action={verifyAction} className="grid gap-4">
        <input type="hidden" name="email" value={requestState.email ?? ""} />
        <Field id="code" label={t("code")} error={verifyState.error}>
          <Input
            id="code"
            name="code"
            inputMode="numeric"
            autoComplete="one-time-code"
            pattern="[0-9]{6}"
            maxLength={6}
            required
            autoFocus
            className="h-15 rounded-2xl bg-background text-center font-heading text-3xl font-bold tracking-[0.45em]"
            aria-invalid={Boolean(verifyState.error)}
          />
        </Field>
        <Button type="submit" size="lg" disabled={verifying} className="h-14 rounded-[18px]">
          {verifying ? t("verifying") : t("verify")}
        </Button>
      </form>
      <Button type="button" variant="link" className="text-foreground underline underline-offset-4" onClick={() => setEditingEmail(true)}>
        {t("useDifferentEmail")}
      </Button>
    </div>
  );
}
