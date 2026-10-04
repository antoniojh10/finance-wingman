"use client";

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
            required
            autoFocus
            defaultValue={requestState.email}
            aria-invalid={Boolean(requestState.error)}
          />
        </Field>
        <Button type="submit" size="lg" disabled={requesting}>
          {requesting ? t("sending") : t("sendLink")}
        </Button>
      </form>
    );
  }

  return (
    <div className="grid gap-4">
      <div>
        <h2 className="font-medium">{t("checkEmail")}</h2>
        <p className="text-sm text-muted-foreground">{t("sentTo", { email: requestState.email ?? "" })}</p>
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
            className="h-11 text-center font-mono text-xl tracking-[0.5em]"
            aria-invalid={Boolean(verifyState.error)}
          />
        </Field>
        <Button type="submit" size="lg" disabled={verifying}>
          {verifying ? t("verifying") : t("verify")}
        </Button>
      </form>
      <Button type="button" variant="ghost" onClick={() => setEditingEmail(true)}>
        {t("useDifferentEmail")}
      </Button>
    </div>
  );
}
