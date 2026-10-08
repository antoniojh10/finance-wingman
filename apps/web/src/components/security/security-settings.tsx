"use client";

import { BotIcon, LogOutIcon, MonitorIcon, SmartphoneIcon, UnplugIcon } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";

import { disconnectApp, revokeOtherSessions, revokeSession } from "@/app/actions/security";
import { ConfirmAction } from "@/components/confirm-action";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { describeUserAgent } from "@/lib/user-agent";

export type BrowserSession = {
  id: string;
  user_agent: string;
  created_at: string;
  last_used_at: string;
  current: boolean;
};

export type ConnectedApp = {
  id: string;
  client_name: string;
  workspace?: { id: string; name: string };
  connected_at: string;
  last_used_at: string;
};

export type SecuritySettingsProps = {
  sessions: BrowserSession[];
  connections: ConnectedApp[];
};

/** Security page body: browser sessions and connected apps. */
export function SecuritySettings({ sessions, connections }: SecuritySettingsProps) {
  return (
    <div className="grid gap-5 lg:grid-cols-2 lg:items-start">
      <SessionList sessions={sessions} />
      <ConnectionList connections={connections} />
    </div>
  );
}

function useDeviceName() {
  const t = useTranslations("security");
  return (userAgent: string) => {
    const { browser, os } = describeUserAgent(userAgent);
    if (browser && os) return t("device", { browser, os });
    return browser ?? os ?? (userAgent ? t("unknownBrowser") : t("unknownDevice"));
  };
}

function SessionList({ sessions }: { sessions: BrowserSession[] }) {
  const t = useTranslations("security");
  const format = useFormatter();
  const deviceName = useDeviceName();
  const hasOthers = sessions.some((session) => !session.current);

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{t("sessions")}</h2>
        </CardTitle>
        <CardDescription>{t("sessionsDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <ul className="grid gap-2" aria-label={t("sessions")}>
          {sessions.map((session) => {
            const name = deviceName(session.user_agent);
            const Icon = describeUserAgent(session.user_agent).mobile ? SmartphoneIcon : MonitorIcon;
            return (
              <li
                key={session.id}
                data-testid="session-row"
                className="flex items-center gap-3 rounded-2xl bg-muted/50 px-3 py-2.5"
              >
                <Icon className="size-5 shrink-0 text-muted-foreground" aria-hidden />
                <div className="grid min-w-0 flex-1">
                  <span className="flex min-w-0 items-center gap-2">
                    <span className="truncate text-sm font-semibold" title={session.user_agent || undefined}>
                      {name}
                    </span>
                    {session.current && <Badge variant="secondary">{t("thisDevice")}</Badge>}
                  </span>
                  <span className="truncate text-xs text-muted-foreground">
                    {t("lastActive", { date: format.dateTime(new Date(session.last_used_at), { dateStyle: "medium", timeStyle: "short" }) })}
                    {" · "}
                    {t("signedIn", { date: format.dateTime(new Date(session.created_at), { dateStyle: "medium" }) })}
                  </span>
                </div>
                {!session.current && (
                  <ConfirmAction
                    trigger={
                      <Button variant="ghost" size="icon" aria-label={t("signOutLabel", { device: name })}>
                        <LogOutIcon />
                      </Button>
                    }
                    title={t("signOutTitle", { device: name })}
                    description={t("signOutDescription")}
                    confirmLabel={t("signOut")}
                    successMessage={t("signedOut")}
                    action={() => revokeSession(session.id)}
                  />
                )}
              </li>
            );
          })}
        </ul>
        {hasOthers && (
          <ConfirmAction
            trigger={
              <Button variant="outline" size="lg" className="w-full">
                <LogOutIcon />
                {t("signOutOthers")}
              </Button>
            }
            title={t("signOutOthersTitle")}
            description={t("signOutOthersDescription")}
            confirmLabel={t("signOutOthers")}
            successMessage={t("signedOutOthers")}
            action={revokeOtherSessions}
          />
        )}
      </CardContent>
    </Card>
  );
}

function ConnectionList({ connections }: { connections: ConnectedApp[] }) {
  const t = useTranslations("security");
  const format = useFormatter();

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{t("connections")}</h2>
        </CardTitle>
        <CardDescription>{t("connectionsDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {connections.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("noConnections")}</p>
        ) : (
          <ul className="grid gap-2" aria-label={t("connections")}>
            {connections.map((connection) => {
              const name = connection.client_name || t("unnamedApp");
              // Each detail is one "·"-separated item under the app's name.
              const details = [
                connection.workspace?.name ?? t("noWorkspace"),
                t("connectedAt", { date: format.dateTime(new Date(connection.connected_at), { dateStyle: "medium" }) }),
                t("lastActive", { date: format.dateTime(new Date(connection.last_used_at), { dateStyle: "medium", timeStyle: "short" }) }),
              ];
              return (
                <li
                  key={connection.id}
                  data-testid="connection-row"
                  className="flex items-center gap-3 rounded-2xl bg-muted/50 px-3 py-2.5"
                >
                  <BotIcon className="size-5 shrink-0 text-muted-foreground" aria-hidden />
                  <div className="grid min-w-0 flex-1">
                    <span className="truncate text-sm font-semibold">{name}</span>
                    <span className="text-xs text-muted-foreground">{details.join(" · ")}</span>
                  </div>
                  <ConfirmAction
                    trigger={
                      <Button variant="ghost" size="icon" aria-label={t("disconnectLabel", { name })}>
                        <UnplugIcon />
                      </Button>
                    }
                    title={t("disconnectTitle", { name })}
                    description={t("disconnectDescription", { name })}
                    confirmLabel={t("disconnect")}
                    successMessage={t("disconnected")}
                    action={() => disconnectApp(connection.id)}
                  />
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
