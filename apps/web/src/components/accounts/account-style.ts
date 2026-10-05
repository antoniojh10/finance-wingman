import { BanknoteIcon, CreditCardIcon, LandmarkIcon, PiggyBankIcon, TrendingUpIcon, WalletIcon, type LucideIcon } from "lucide-react";

export type AccountType = "checking" | "savings" | "credit_card" | "cash" | "investment" | "other";

/** Color and icon that identify each account type across the app. */
export const accountStyles: Record<AccountType, { color: string; icon: LucideIcon }> = {
  checking: { color: "#2f8cff", icon: LandmarkIcon },
  savings: { color: "#6d4aff", icon: PiggyBankIcon },
  credit_card: { color: "#ff6b4a", icon: CreditCardIcon },
  cash: { color: "#ffb020", icon: BanknoteIcon },
  investment: { color: "#14b8a6", icon: TrendingUpIcon },
  other: { color: "#9aa0b8", icon: WalletIcon },
};
