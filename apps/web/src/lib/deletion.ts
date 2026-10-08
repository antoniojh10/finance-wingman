/** Days a scheduled deletion can be cancelled before it is carried out (set by the API). */
export const DELETION_GRACE_DAYS = 7;

/** When a deletion scheduled at `now` (epoch milliseconds) will be carried out. */
export function deletionDate(now: number): Date {
  return new Date(now + DELETION_GRACE_DAYS * 24 * 60 * 60 * 1000);
}
