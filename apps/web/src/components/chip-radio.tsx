import { useId } from "react";

import { cn } from "@/lib/utils";

export type ChipOption = { value: string; label: string; detail?: string; leading?: React.ReactNode };

/**
 * A labelled group of native radio buttons shown as tappable chips. Native
 * radios keep arrow-key navigation and submit with the form.
 */
export function ChipRadioGroup({
  name,
  label,
  options,
  value,
  defaultValue,
  onChange,
  error,
}: {
  name: string;
  label: string;
  options: ChipOption[];
  value?: string;
  defaultValue?: string;
  onChange?: (value: string) => void;
  error?: string;
}) {
  const id = useId();
  return (
    <div className="grid gap-2.5">
      <p id={`${id}-label`} className="px-1 text-[13px] font-bold text-muted-foreground">
        {label}
      </p>
      <div role="radiogroup" aria-labelledby={`${id}-label`} className="flex flex-wrap gap-2">
        {options.map((option) => (
          <label
            key={option.value}
            className={cn(
              "flex min-h-11 cursor-pointer items-center gap-2 rounded-[14px] border-2 border-border bg-card px-3.5 text-sm font-semibold transition-colors select-none",
              "hover:border-foreground/25 has-checked:border-primary has-checked:bg-primary/8 has-focus-visible:ring-3 has-focus-visible:ring-ring/50",
              option.leading && "pl-1.5",
            )}
          >
            <input
              type="radio"
              name={name}
              value={option.value}
              className="sr-only"
              {...(value !== undefined
                ? { checked: value === option.value, onChange: () => onChange?.(option.value) }
                : { defaultChecked: defaultValue === option.value })}
            />
            {option.leading}
            <span className="flex flex-col leading-tight">
              {option.label}
              {option.detail && " "}
              {option.detail && <span className="text-[11.5px] font-semibold text-muted-foreground">{option.detail}</span>}
            </span>
          </label>
        ))}
      </div>
      {error && (
        <p className="px-1 text-xs text-destructive" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}

/** A labelled pill switch made of native radio buttons, for a few short options. */
export function SegmentedRadioGroup({
  name,
  label,
  options,
  defaultValue,
}: {
  name: string;
  label: string;
  options: { value: string; label: string }[];
  defaultValue?: string;
}) {
  const id = useId();
  return (
    <div className="grid gap-1.5">
      <p id={`${id}-label`} className="text-[13px] font-bold text-muted-foreground">
        {label}
      </p>
      <div
        role="radiogroup"
        aria-labelledby={`${id}-label`}
        className="grid gap-1 rounded-2xl bg-muted p-1"
        style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }}
      >
        {options.map((option) => (
          <label
            key={option.value}
            className="flex min-h-11 cursor-pointer items-center justify-center rounded-xl text-sm font-bold text-muted-foreground transition-colors select-none hover:text-foreground has-checked:bg-card has-checked:text-foreground has-checked:shadow-sm has-focus-visible:ring-3 has-focus-visible:ring-ring/50"
          >
            <input type="radio" name={name} value={option.value} defaultChecked={defaultValue === option.value} className="sr-only" />
            {option.label}
          </label>
        ))}
      </div>
    </div>
  );
}
