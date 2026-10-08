"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";

import { AccountDialog } from "@/components/accounts/account-dialog";

/** Query flag that makes the accounts page open the new-account form on load. */
export const NEW_ACCOUNT_PARAM = "new";

/** Link to the accounts page with the new-account form open. */
export const NEW_ACCOUNT_HREF = `/accounts?${NEW_ACCOUNT_PARAM}=1`;

/**
 * The create-account dialog. It opens on its own when the URL carries
 * `?new=1` and drops that flag (keeping other params) when it closes, so a
 * refresh or going back doesn't reopen it.
 */
export function NewAccountDialog(props: Omit<React.ComponentProps<typeof AccountDialog>, "open" | "onOpenChange">) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [opened, setOpened] = useState(false);
  const flagged = searchParams.get(NEW_ACCOUNT_PARAM) === "1";

  return (
    <AccountDialog
      {...props}
      open={opened || flagged}
      onOpenChange={(next) => {
        setOpened(next);
        if (!next && flagged) {
          const params = new URLSearchParams(searchParams.toString());
          params.delete(NEW_ACCOUNT_PARAM);
          const qs = params.toString();
          router.replace(qs ? `${pathname}?${qs}` : pathname);
        }
      }}
    />
  );
}
