"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useActionState } from "react";

import { verifyLink } from "@/app/actions/auth";
import { Button } from "@/components/ui/button";

/**
 * The link is redeemed only when the user presses Continue, so email
 * scanners that prefetch links cannot consume it.
 */
export function VerifyLinkForm({ token }: { token: string }) {
  const t = useTranslations("auth");
  const [state, action, pending] = useActionState(verifyLink, {});

  if (state.error) {
    return (
      <div className="grid gap-4">
        <p className="text-sm text-destructive" role="alert">
          {state.error}
        </p>
        <Button nativeButton={false} render={<Link href="/login" />} variant="outline">
          {t("backToLogin")}
        </Button>
      </div>
    );
  }

  return (
    <form action={action}>
      <input type="hidden" name="token" value={token} />
      <Button type="submit" size="lg" className="w-full" disabled={pending}>
        {pending ? t("verifying") : t("continue")}
      </Button>
    </form>
  );
}
