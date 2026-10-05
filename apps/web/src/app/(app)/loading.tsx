import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return (
    <div className="grid gap-5" aria-busy="true">
      <Skeleton className="h-11 w-56 rounded-2xl" />
      <div className="grid gap-5 lg:grid-cols-2">
        <Skeleton className="h-72 rounded-[28px]" />
        <Skeleton className="h-72 rounded-3xl" />
      </div>
      <Skeleton className="h-64 rounded-3xl" />
    </div>
  );
}
