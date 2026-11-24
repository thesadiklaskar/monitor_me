import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

type AlertProps = ComponentProps<"div"> & { variant?: "default" | "destructive" | "warning" };

function Alert({ className, variant = "default", ...props }: AlertProps) {
  const variantClass = {
    default: "border-[rgba(127,224,193,.25)] bg-[rgba(127,224,193,.07)] text-[var(--foreground)]",
    destructive: "border-[rgba(242,143,145,.32)] bg-[rgba(126,39,52,.17)] text-[#ffb5b8]",
    warning: "border-[rgba(235,183,95,.32)] bg-[rgba(135,92,25,.14)] text-[#f2d29a]"
  }[variant];
  return <div data-slot="alert" role="alert" className={cn("relative w-full rounded-lg border px-4 py-3 text-xs leading-relaxed", variantClass, className)} {...props} />;
}

export { Alert };
