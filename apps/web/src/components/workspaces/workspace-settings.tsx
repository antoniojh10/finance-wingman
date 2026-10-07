"use client";

import { LogOutIcon, MailIcon, ShieldIcon, UserIcon, UserMinusIcon, XIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { useFormatter, useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import {
  inviteMember,
  leaveWorkspace,
  removeMember,
  renameWorkspace,
  revokeInvitation,
  setMemberRole,
} from "@/app/actions/workspaces";
import { Avatar } from "@/components/brand";
import { ConfirmAction } from "@/components/confirm-action";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { RowActions, type RowAction } from "@/components/row-actions";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";

type Role = "owner" | "member";

export type WorkspaceSettingsProps = {
  workspace: { id: string; name: string; role: Role };
  userId: string;
  members: { user_id: string; email: string; name: string; role: Role }[];
  /** Open invitations; only loaded for owners. */
  invitations: { id: string; email: string; role: Role; invited_by?: string; expires_at: string }[];
};

/** Workspace page body: name, members, invitations and leaving. */
export function WorkspaceSettings({ workspace, userId, members, invitations }: WorkspaceSettingsProps) {
  const t = useTranslations("workspaces");
  const isOwner = workspace.role === "owner";

  return (
    <div className="grid gap-5 lg:grid-cols-2 lg:items-start">
      <div className="grid gap-5">
        <Card>
          <CardHeader>
            <CardTitle>
              <h2>{t("general")}</h2>
            </CardTitle>
            {!isOwner && <CardDescription>{t("ownerOnly")}</CardDescription>}
          </CardHeader>
          <CardContent>
            <RenameForm workspace={workspace} disabled={!isOwner} />
          </CardContent>
        </Card>
        {isOwner && (
          <Card>
            <CardHeader>
              <CardTitle>
                <h2>{t("inviteTitle")}</h2>
              </CardTitle>
              <CardDescription>{t("inviteDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <InviteForm workspaceId={workspace.id} />
            </CardContent>
          </Card>
        )}
      </div>
      <div className="grid gap-5">
        <Card>
          <CardHeader>
            <CardTitle>
              <h2>{t("members")}</h2>
            </CardTitle>
            <CardDescription>{t("roleDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <MemberList workspaceId={workspace.id} userId={userId} members={members} canManage={isOwner} />
          </CardContent>
        </Card>
        {isOwner && (
          <Card>
            <CardHeader>
              <CardTitle>
                <h2>{t("invitations")}</h2>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <InvitationList workspaceId={workspace.id} invitations={invitations} />
            </CardContent>
          </Card>
        )}
        <LeaveWorkspace workspace={workspace} userId={userId} />
      </div>
    </div>
  );
}

function RenameForm({ workspace, disabled }: { workspace: { id: string; name: string }; disabled: boolean }) {
  const t = useTranslations();
  const router = useRouter();
  const { state, onSubmit, pending } = useFormAction(renameWorkspace, () => {
    toast.success(t("workspaces.renamed"));
    router.refresh();
  });

  return (
    <form onSubmit={onSubmit} className="grid gap-4" noValidate>
      <input type="hidden" name="id" value={workspace.id} />
      <Field id="workspace-name" label={t("workspaces.name")} error={state.fieldErrors?.name}>
        <Input
          id="workspace-name"
          name="name"
          defaultValue={workspace.name}
          maxLength={100}
          required
          disabled={disabled}
          className="bg-background"
        />
      </Field>
      {state.message && !state.ok && !state.fieldErrors && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      {!disabled && (
        <Button type="submit" size="lg" disabled={pending}>
          {pending ? t("common.saving") : t("common.save")}
        </Button>
      )}
    </form>
  );
}

function RoleOptions() {
  const t = useTranslations("workspaces.roles");
  return (
    <>
      <option value="member">{t("member")}</option>
      <option value="owner">{t("owner")}</option>
    </>
  );
}

function InviteForm({ workspaceId }: { workspaceId: string }) {
  const t = useTranslations();
  const [formKey, setFormKey] = useState(0);
  const { state, onSubmit, pending } = useFormAction(inviteMember, () => {
    toast.success(t("workspaces.invited"));
    // Clear the email for the next invitation.
    setFormKey((k) => k + 1);
  });

  return (
    <form key={formKey} onSubmit={onSubmit} className="grid gap-4" noValidate>
      <input type="hidden" name="workspace_id" value={workspaceId} />
      <Field id="invite-email" label={t("workspaces.email")} error={state.fieldErrors?.email}>
        <Input
          id="invite-email"
          name="email"
          type="email"
          autoComplete="off"
          required
          aria-invalid={Boolean(state.fieldErrors?.email)}
          className="bg-background"
        />
      </Field>
      <Field id="invite-role" label={t("workspaces.role")}>
        <NativeSelect id="invite-role" name="role" defaultValue="member">
          <RoleOptions />
        </NativeSelect>
      </Field>
      {state.message && !state.ok && !state.fieldErrors && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <Button type="submit" size="lg" disabled={pending}>
        <MailIcon />
        {t("workspaces.invite")}
      </Button>
    </form>
  );
}

function MemberList({
  workspaceId,
  userId,
  members,
  canManage,
}: {
  workspaceId: string;
  userId: string;
  members: WorkspaceSettingsProps["members"];
  canManage: boolean;
}) {
  const t = useTranslations("workspaces");
  const router = useRouter();
  const [removing, setRemoving] = useState<WorkspaceSettingsProps["members"][number] | null>(null);

  async function changeRole(member: WorkspaceSettingsProps["members"][number], role: Role) {
    const result = await setMemberRole(workspaceId, member.user_id, role);
    if (result.ok) {
      toast.success(t("roleChanged"));
      router.refresh();
    } else {
      toast.error(result.message);
    }
  }

  return (
    <>
      <ul className="grid gap-2">
        {members.map((member) => {
          const label = member.name || member.email;
          const self = member.user_id === userId;
          const actions: RowAction[] = [
            member.role === "owner"
              ? { label: t("makeMember"), icon: UserIcon, onSelect: () => changeRole(member, "member") }
              : { label: t("makeOwner"), icon: ShieldIcon, onSelect: () => changeRole(member, "owner") },
          ];
          if (!self) {
            actions.push({ label: t("remove"), icon: UserMinusIcon, onSelect: () => setRemoving(member), destructive: true });
          }
          return (
            <li key={member.user_id} className="flex items-center gap-3 rounded-2xl bg-muted/50 px-3 py-2.5">
              <Avatar name={label} className="size-9" />
              <div className="grid min-w-0 flex-1">
                <span className="truncate text-sm font-semibold">
                  {label}
                  {self && <span className="font-normal text-muted-foreground"> ({t("you")})</span>}
                </span>
                {member.name && <span className="truncate text-xs text-muted-foreground">{member.email}</span>}
              </div>
              <Badge variant={member.role === "owner" ? "default" : "secondary"}>{t(`roles.${member.role}`)}</Badge>
              {canManage && <RowActions actions={actions} />}
            </li>
          );
        })}
      </ul>
      <ConfirmAction
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={t("removeTitle", { name: removing ? removing.name || removing.email : "" })}
        description={t("removeDescription")}
        confirmLabel={t("remove")}
        successMessage={t("removed")}
        action={async () => {
          const result = await removeMember(workspaceId, removing!.user_id);
          if (result.ok) router.refresh();
          return result;
        }}
      />
    </>
  );
}

function InvitationList({ workspaceId, invitations }: { workspaceId: string; invitations: WorkspaceSettingsProps["invitations"] }) {
  const t = useTranslations("workspaces");
  const format = useFormatter();

  if (invitations.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("noInvitations")}</p>;
  }
  return (
    <ul className="grid gap-2">
      {invitations.map((invitation) => (
        <li key={invitation.id} className="flex items-center gap-3 rounded-2xl bg-muted/50 px-3 py-2.5">
          <MailIcon className="size-5 shrink-0 text-muted-foreground" aria-hidden />
          <div className="grid min-w-0 flex-1">
            <span className="truncate text-sm font-semibold">{invitation.email}</span>
            <span className="truncate text-xs text-muted-foreground">
              {t(`roles.${invitation.role}`)} ·{" "}
              {t("expires", { date: format.dateTime(new Date(invitation.expires_at), { dateStyle: "medium" }) })}
            </span>
          </div>
          <ConfirmAction
            trigger={
              <Button variant="ghost" size="icon" aria-label={t("revoke")}>
                <XIcon />
              </Button>
            }
            title={t("revokeTitle", { email: invitation.email })}
            description={t("revokeDescription")}
            confirmLabel={t("revoke")}
            successMessage={t("revoked")}
            action={() => revokeInvitation(workspaceId, invitation.id)}
          />
        </li>
      ))}
    </ul>
  );
}

function LeaveWorkspace({ workspace, userId }: { workspace: { id: string; name: string }; userId: string }) {
  const t = useTranslations("workspaces");
  return (
    <ConfirmAction
      trigger={
        <Button variant="destructive" size="lg" className="w-full border-2 border-destructive/25">
          <LogOutIcon />
          {t("leave")}
        </Button>
      }
      title={t("leaveTitle", { name: workspace.name })}
      description={t("leaveDescription")}
      confirmLabel={t("leave")}
      successMessage={t("left")}
      action={() => leaveWorkspace(workspace.id, userId)}
    />
  );
}
