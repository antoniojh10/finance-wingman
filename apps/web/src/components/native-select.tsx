import { ChevronDownIcon } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * A styled native <select>. Native controls give the best experience on
 * phones (system pickers) and submit with plain forms.
 */
export function NativeSelect({ className, children, ...props }: React.ComponentProps<"select">) {
  return (
    <div className="relative">
      <select
        className={cn(
          "h-11 w-full appearance-none rounded-xl border border-input bg-card py-1 pr-9 pl-3.5 text-base outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive md:text-sm dark:bg-input/30 [&_option]:bg-popover [&_option]:text-popover-foreground",
          className,
        )}
        {...props}
      >
        {children}
      </select>
      <ChevronDownIcon className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-muted-foreground" />
    </div>
  );
}
