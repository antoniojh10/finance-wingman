import { TriangleAlertIcon } from "lucide-react";
import Link from "next/link";
import { useFormatter, useTranslations } from "next-intl";

/**
 * Banners shown on every page while a deletion is scheduled, linking to
 * where it can be cancelled.
 */
export function DeletionNotices({ workspace }: { workspace?: { name: string; deletion_scheduled_for?: string } }) {
  const t = useTranslations();
  const format = useFormatter();
  const date = (iso: string) => format.dateTime(new Date(iso), { dateStyle: "long" });
  const notices: { key: string; text: string; href: string; link: string }[] = [];
  if (workspace?.deletion_scheduled_for) {
    notices.push({
      key: "workspace",
      text: t("workspaces.deletionBanner", { name: workspace.name, date: date(workspace.deletion_scheduled_for) }),
      href: "/settings/workspace",
      link: t("workspaces.deletionBannerLink"),
    });
  }
  if (notices.length === 0) {
    return null;
  }
  return (
    <div className="mb-4 grid gap-2">
      {notices.map((notice) => (
        <p
          key={notice.key}
          role="status"
          className="flex flex-wrap items-center gap-2 rounded-2xl bg-destructive/10 px-4 py-3 text-sm text-destructive"
        >
          <TriangleAlertIcon className="size-4 shrink-0" aria-hidden />
          <span className="min-w-0 flex-1">{notice.text}</span>
          <Link href={notice.href} className="font-semibold underline underline-offset-4">
            {notice.link}
          </Link>
        </p>
      ))}
    </div>
  );
}
