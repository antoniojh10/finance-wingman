import { useTranslations } from "next-intl";
import type { CSSProperties } from "react";

import { accountStyles, type AccountType } from "@/components/accounts/account-style";
import { Money } from "@/components/money";

export type AccountSummary = {
  id: string;
  name: string;
  type: AccountType;
  currency: string;
  minor_units: number;
  balance: number;
};

/** Account balances as tinted cards: a swipeable row on phones, a grid on wider screens. */
export function AccountStrip({ accounts }: { accounts: AccountSummary[] }) {
  const t = useTranslations("accounts");
  return (
    <ul className="-mx-4 flex snap-x gap-3 overflow-x-auto px-4 pb-1 md:mx-0 md:grid md:grid-cols-3 md:overflow-visible md:px-0 xl:grid-cols-4">
      {accounts.map((a) => (
        <li
          key={a.id}
          className="grid w-40 shrink-0 snap-start gap-4.5 rounded-[22px] bg-[color-mix(in_oklab,var(--tile)_16%,var(--card))] p-4 md:w-auto"
          style={{ "--tile": accountStyles[a.type].color } as CSSProperties}
        >
          <div className="flex items-center justify-between gap-2 text-xs text-[color-mix(in_oklab,var(--tile)_55%,var(--foreground))]">
            <span className="font-bold tracking-wide">{a.currency}</span>
            <span className="truncate font-semibold">{t(`types.${a.type}`)}</span>
          </div>
          <div className="grid gap-0.5">
            <p className="truncate text-sm font-semibold">{a.name}</p>
            <p className="font-heading text-[19px] font-bold">
              <Money
                amount={a.balance}
                currency={a.currency}
                minorUnits={a.minor_units}
                className={a.balance < 0 ? "text-expense" : undefined}
              />
            </p>
          </div>
        </li>
      ))}
    </ul>
  );
}
