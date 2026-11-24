import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

function Skeleton({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="skeleton" aria-hidden="true" className={cn("animate-pulse rounded-md bg-[var(--surface-raised)]", className)} {...props} />;
}

export { Skeleton };
