"use client";

import { XIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { buttonVariants } from "@/components/ui/button";

import { TransactionForm } from "./transaction-dialog";
import type { AccountOption, CategoryOption } from "./types";
import { useSavedToast } from "./use-saved-toast";
import { NEW_ACCOUNT_HREF } from "@/components/accounts/new-account-dialog";

/** Full-page form for adding a transaction, returning to `returnTo` when done. */
export function NewTransactionView({
  accounts,
  categories,
  defaultDate,
  returnTo,
}: {
  accounts: AccountOption[];
  categories: CategoryOption[];
  defaultDate: string;
  returnTo: string;
}) {
  const t = useTranslations();
  const router = useRouter();
  const announceSaved = useSavedToast();

  return (
    <div className="mx-auto grid max-w-lg gap-6">
      <div className="flex items-center gap-2">
        <Link
          href={returnTo}
          aria-label={t("common.close")}
          className={buttonVariants({ variant: "secondary", size: "icon-lg", className: "bg-card" })}
        >
          <XIcon />
        </Link>
        <h1 className="text-2xl font-bold tracking-tight">{t("transactions.addTitle")}</h1>
      </div>
      {accounts.length === 0 ? (
        <p className="rounded-2xl bg-card px-5 py-4 text-sm">
          {t("transactions.needAccount")}{" "}
          <Link href={NEW_ACCOUNT_HREF} className="font-semibold text-primary underline underline-offset-4">
            {t("accounts.add")}
          </Link>
        </p>
      ) : (
        <TransactionForm
          accounts={accounts}
          categories={categories}
          defaultDate={defaultDate}
          onSaved={({ transactionId }) => {
            void announceSaved(transactionId);
            router.push(returnTo);
          }}
          onCancel={() => router.push(returnTo)}
        />
      )}
    </div>
  );
}
