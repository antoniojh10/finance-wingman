import { AppShell } from "@/components/app-shell";
import { getCurrentUser } from "@/lib/session";

export default async function AuthenticatedLayout({ children }: { children: React.ReactNode }) {
  const user = await getCurrentUser();
  return <AppShell userName={user.name || user.email}>{children}</AppShell>;
}
