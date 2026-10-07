const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** A workspace member who can own accounts. */
export type OwnerOption = { id: string; name: string };

/** Value of the owner filter and form field for shared accounts. */
export const SHARED = "shared";

/** A member's display name: their name, or their email when they have none. */
export function ownerName(owner: { name: string; email: string }): string {
  return owner.name.trim() || owner.email;
}

export function toOwnerOption(member: { user_id: string; name: string; email: string }): OwnerOption {
  return { id: member.user_id, name: ownerName(member) };
}

/**
 * Names an account with its owner, e.g. "BNP (Ana)", so members' accounts
 * with the same name can be told apart. Shared accounts keep their name.
 */
export function accountLabel(name: string, owner: { name: string; email: string } | null | undefined, showOwner: boolean): string {
  return showOwner && owner ? `${name} (${ownerName(owner)})` : name;
}

/** Sanitizes an owner filter: a user id or "shared"; anything else means all. */
export function parseOwner(value: string | string[] | undefined): string | undefined {
  const owner = (Array.isArray(value) ? value[0] : value)?.trim();
  return owner && (owner === SHARED || uuidPattern.test(owner)) ? owner : undefined;
}
