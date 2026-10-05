import { Brand } from "@/components/brand";
import { cn } from "@/lib/utils";

// Decorative mosaic of shapes in the category palette.
const tiles = [
  "rounded-full bg-primary",
  "rounded-[22px] bg-lime",
  "rounded-t-full bg-[#ff6b4a]",
  "rounded-tr-full rounded-bl-full bg-hero",
  "rounded-tl-full rounded-br-full bg-[#ffb020]",
  "rounded-full bg-hero",
  "rounded-[22px] bg-[#2f8cff]",
  "rounded-b-full bg-[#f05aa6]",
  "rounded-[22px] bg-[#14b8a6]",
  "rounded-tl-full bg-[#6d4aff]",
  "rounded-full bg-expense-soft",
  "rounded-r-full bg-lime",
];

/** Shared frame of the sign-in pages: mosaic, brand, then the page content. */
export function AuthLayout({ appName, children }: { appName: string; children: React.ReactNode }) {
  return (
    <main className="flex min-h-dvh justify-center px-4 pt-7 pb-10 sm:items-center">
      <div className="grid w-full max-w-sm content-start gap-7">
        <div aria-hidden className="grid grid-cols-4 gap-2.5">
          {tiles.map((tile, index) => (
            <div key={index} className={cn("relative aspect-square", tile)}>
              {index === 5 && <span className="absolute inset-[28%] rounded-full bg-lime" />}
            </div>
          ))}
        </div>
        <Brand name={appName} className="px-1" />
        <div className="grid gap-5 px-1">{children}</div>
      </div>
    </main>
  );
}

export function AuthHeading({ title, description }: { title: string; description?: string }) {
  return (
    <div className="grid gap-2">
      <h1 className="text-[34px] leading-[1.05] font-bold tracking-tight">{title}</h1>
      {description && <p className="text-[15px] leading-relaxed text-muted-foreground">{description}</p>}
    </div>
  );
}
