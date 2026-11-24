import type { ComponentProps } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex w-fit shrink-0 items-center gap-1 rounded-full border px-2.5 py-1 text-[10px] font-semibold leading-none tracking-wide transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)]",
  {
    variants: {
      variant: {
        default: "border-[rgba(127,224,193,.25)] bg-[rgba(127,224,193,.1)] text-[#a9ecd7]",
        secondary: "border-[var(--border)] bg-[var(--surface-raised)] text-[var(--foreground)]",
        outline: "border-[var(--border)] bg-transparent text-[var(--muted)]",
        destructive: "border-[rgba(242,143,145,.32)] bg-[rgba(126,39,52,.17)] text-[#ffb5b8]",
        warning: "border-[rgba(235,183,95,.32)] bg-[rgba(135,92,25,.14)] text-[#f2d29a]"
      }
    },
    defaultVariants: { variant: "default" }
  }
);

type BadgeProps = ComponentProps<"span"> & VariantProps<typeof badgeVariants>;

function Badge({ className, variant, ...props }: BadgeProps) {
  return <span data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { Badge, badgeVariants };
