"use client";

import { ArchiveIcon, ArchiveRestoreIcon, PencilIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { createGroup, createStore, renameGroup, renameStore, setStoreArchived } from "@/app/actions/groceries-catalog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import type { FormState } from "@/lib/forms";
import type { GroceryStore, ProductGroup } from "@/lib/groceries-products";

type Action = (state: FormState, formData: FormData) => Promise<FormState>;

function RenameForm({ id, name, action, label, onDone }: { id: string; name: string; action: Action; label: string; onDone: () => void }) {
  const t = useTranslations("groceries.products.admin");
  const { state, onSubmit, pending } = useFormAction(action, () => {
    toast.success(t("renamed"));
    onDone();
  });
  const error = state.fieldErrors?.name ?? (!state.ok ? state.message : undefined);

  return (
    <form onSubmit={onSubmit} noValidate className="grid min-w-0 flex-1 gap-1.5">
      <input type="hidden" name="id" value={id} />
      <div className="flex gap-2">
        <Input name="name" defaultValue={name} aria-label={label} maxLength={80} autoComplete="off" autoFocus aria-invalid={Boolean(error)} className="h-9 min-w-0 flex-1" />
        <Button type="submit" size="sm" disabled={pending}>
          {t("save")}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onDone}>
          {t("cancel")}
        </Button>
      </div>
      {error && (
        <p className="text-xs text-destructive" role="alert">
          {error}
        </p>
      )}
    </form>
  );
}

function AddForm({ action, label, placeholder, withFood }: { action: Action; label: string; placeholder: string; withFood?: boolean }) {
  const t = useTranslations("groceries.products.admin");
  const [key, setKey] = useState(0);
  const { state, onSubmit, pending } = useFormAction(action, () => {
    toast.success(t("added"));
    setKey((value) => value + 1);
  });
  const error = state.fieldErrors?.name ?? (!state.ok ? state.message : undefined);

  return (
    <form key={key} onSubmit={onSubmit} noValidate className="grid gap-2 border-t pt-3">
      <div className="flex gap-2">
        <Input name="name" aria-label={label} placeholder={placeholder} maxLength={80} autoComplete="off" aria-invalid={Boolean(error)} className="h-9 min-w-0 flex-1" />
        <Button type="submit" size="sm" variant="outline" disabled={pending}>
          {t("add")}
        </Button>
      </div>
      {withFood && (
        <label className="flex items-center gap-2 text-sm text-muted-foreground">
          <input type="checkbox" name="is_food" defaultChecked className="size-4 accent-primary" />
          {t("isFood")}
        </label>
      )}
      {error && (
        <p className="text-xs text-destructive" role="alert">
          {error}
        </p>
      )}
    </form>
  );
}

function StoreRow({ store }: { store: GroceryStore }) {
  const t = useTranslations("groceries.products.admin");
  const [editing, setEditing] = useState(false);
  const [pending, startTransition] = useTransition();

  function toggleArchived() {
    startTransition(async () => {
      const result = await setStoreArchived(store.id, !store.archived);
      if (result.ok) {
        toast.success(store.archived ? t("restored") : t("archived"));
      } else {
        toast.error(result.message);
      }
    });
  }

  return (
    <li className="flex min-h-11 items-center gap-2 py-1" data-testid="store-row">
      {editing ? (
        <RenameForm id={store.id} name={store.name} action={renameStore} label={t("storeName")} onDone={() => setEditing(false)} />
      ) : (
        <>
          <span className={store.archived ? "min-w-0 flex-1 truncate text-muted-foreground" : "min-w-0 flex-1 truncate font-medium"}>{store.name}</span>
          {store.archived && <Badge variant="outline">{t("archivedChip")}</Badge>}
          <Button variant="ghost" size="icon" aria-label={t("rename", { name: store.name })} onClick={() => setEditing(true)}>
            <PencilIcon />
          </Button>
          <Button variant="ghost" size="icon" aria-label={store.archived ? t("restore", { name: store.name }) : t("archive", { name: store.name })} disabled={pending} onClick={toggleArchived}>
            {store.archived ? <ArchiveRestoreIcon /> : <ArchiveIcon />}
          </Button>
        </>
      )}
    </li>
  );
}

function GroupRow({ group }: { group: ProductGroup }) {
  const t = useTranslations("groceries.products.admin");
  const [editing, setEditing] = useState(false);

  return (
    <li className="flex min-h-11 items-center gap-2 py-1" data-testid="group-row">
      {editing ? (
        <RenameForm id={group.id} name={group.name} action={renameGroup} label={t("groupName")} onDone={() => setEditing(false)} />
      ) : (
        <>
          <i aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ background: group.color ?? "var(--muted-foreground)" }} />
          <span className="min-w-0 flex-1 truncate font-medium">{group.name}</span>
          {!group.is_food && <Badge variant="outline">{t("nonFood")}</Badge>}
          <Button variant="ghost" size="icon" aria-label={t("rename", { name: group.name })} onClick={() => setEditing(true)}>
            <PencilIcon />
          </Button>
        </>
      )}
    </li>
  );
}

/** Compact management of stores (rename, archive) and product groups (rename, add). */
export function CatalogAdmin({ stores, groups }: { stores: GroceryStore[]; groups: ProductGroup[] }) {
  const t = useTranslations("groceries.products.admin");
  const sortedStores = [...stores].sort((a, b) => Number(a.archived) - Number(b.archived) || a.name.localeCompare(b.name));
  const sortedGroups = [...groups].sort((a, b) => a.position - b.position || a.name.localeCompare(b.name));

  return (
    <section aria-labelledby="catalog-admin-title" className="grid gap-4 rounded-3xl border bg-card p-5 md:p-6">
      <div>
        <h2 id="catalog-admin-title" className="text-lg font-bold">
          {t("heading")}
        </h2>
        <p className="mt-0.5 text-sm text-muted-foreground">{t("description")}</p>
      </div>
      <div className="grid items-start gap-6 md:grid-cols-2">
        <div className="grid gap-2" data-testid="stores-admin">
          <h3 className="text-sm font-bold tracking-wide text-muted-foreground uppercase">{t("stores")}</h3>
          {sortedStores.length === 0 ? <p className="text-sm text-muted-foreground">{t("noStores")}</p> : <ul className="divide-y">{sortedStores.map((store) => <StoreRow key={store.id} store={store} />)}</ul>}
          <AddForm action={createStore} label={t("newStore")} placeholder={t("newStore")} />
        </div>
        <div className="grid gap-2" data-testid="groups-admin">
          <h3 className="text-sm font-bold tracking-wide text-muted-foreground uppercase">{t("groups")}</h3>
          <ul className="divide-y">{sortedGroups.map((group) => <GroupRow key={group.id} group={group} />)}</ul>
          <AddForm action={createGroup} label={t("newGroup")} placeholder={t("newGroup")} withFood />
        </div>
      </div>
    </section>
  );
}
