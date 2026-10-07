"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useActionState } from "react";

import { acceptInvitation } from "@/app/actions/workspaces";
import { Button } from "@/components/ui/button";

/**
 * The invitation is accepted only when the user presses the button, so email
 * scanners that prefetch links cannot consume it.
 */
export function AcceptInvitationForm({ token }: { token: string }) {
  const t = useTranslations("invite");
  const [state, action, pending] = useActionState(acceptInvitation, {});

  if (state.error) {
    return <InvalidInvitation message={state.error} />;
  }
  return (
    <form action={action}>
      <input type="hidden" name="token" value={token} />
      <Button type="submit" size="lg" className="h-14 w-full rounded-[18px]" disabled={pending}>
        {pending ? t("accepting") : t("accept")}
      </Button>
    </form>
  );
}

export function InvalidInvitation({ message }: { message: string }) {
  const t = useTranslations("invite");
  return (
    <div className="grid gap-4">
      <p className="text-sm text-destructive" role="alert">
        {message}
      </p>
      <Button nativeButton={false} render={<Link href="/login" />} variant="outline">
        {t("backToLogin")}
      </Button>
    </div>
  );
}
