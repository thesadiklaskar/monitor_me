import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

function Input({ className, type = "text", ...props }: ComponentProps<"input">) {
  return (
    <input
      data-slot="input"
      type={type}
      className={cn("flex h-10 w-full min-w-0 rounded-lg border border-[#304159] bg-[#0b1421] px-3 py-2 text-sm text-[var(--foreground)] shadow-xs outline-none placeholder:text-[#65758d] focus-visible:border-[var(--accent-strong)] focus-visible:ring-[3px] focus-visible:ring-[rgba(76,200,164,.12)] disabled:cursor-not-allowed disabled:opacity-50", className)}
      {...props}
    />
  );
}

export { Input };
