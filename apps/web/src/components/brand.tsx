import { cn } from "@/lib/utils";

/** Two overlapping circles: the app's logo mark. */
export function BrandMark({ className }: { className?: string }) {
  return (
    <span aria-hidden className={cn("relative inline-block h-7 w-10 shrink-0", className)}>
      <span className="absolute top-0 left-0 size-7 rounded-full bg-primary" />
      <span className="absolute top-0 left-3 size-7 rounded-full bg-lime mix-blend-multiply dark:mix-blend-screen" />
    </span>
  );
}

/** Logo mark followed by the app name. */
export function Brand({ name, className }: { name: string; className?: string }) {
  return (
    <span className={cn("flex items-center gap-2.5", className)}>
      <BrandMark />
      <span className="font-heading text-lg font-bold tracking-tight">{name}</span>
    </span>
  );
}

/** A round badge with the first letter of a person's name. */
export function Avatar({ name, className }: { name: string; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        "flex size-11 shrink-0 items-center justify-center rounded-full bg-expense-soft font-heading text-base font-bold text-expense",
        className,
      )}
    >
      {name.trim().charAt(0).toUpperCase() || "?"}
    </span>
  );
}
