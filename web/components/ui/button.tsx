import type { ComponentProps } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-[9px] text-xs font-semibold transition-[background,color,border-color,box-shadow,transform] duration-150 outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--background)] disabled:pointer-events-none disabled:opacity-50 active:translate-y-px",
  {
    variants: {
      variant: {
        default: "border border-[rgba(127,224,193,.35)] bg-[var(--accent)] text-[#071a17] hover:bg-[#9cebd3]",
        secondary: "border border-[#34445c] bg-[#182438] text-[#dce6f2] hover:bg-[#20314b]",
        destructive: "border border-[rgba(242,143,145,.34)] bg-[rgba(242,143,145,.1)] text-[#ffb4b5] hover:bg-[rgba(242,143,145,.18)]",
        outline: "border border-[var(--border)] bg-transparent text-[var(--foreground)] hover:bg-[var(--surface-raised)]",
        ghost: "border border-transparent bg-transparent text-[var(--muted)] hover:bg-[var(--surface-raised)] hover:text-[var(--foreground)]",
        link: "h-auto border-0 bg-transparent p-0 text-[var(--accent)] underline-offset-4 hover:underline"
      },
      size: {
        default: "min-h-9 px-4 py-2",
        sm: "min-h-8 rounded-lg px-3 py-1.5 text-[11px]",
        lg: "min-h-11 rounded-xl px-5 py-3 text-sm",
        icon: "size-9 p-0"
      }
    },
    defaultVariants: { variant: "default", size: "default" }
  }
);

type ButtonProps = ComponentProps<"button"> & VariantProps<typeof buttonVariants>;

function Button({ className, variant, size, type = "button", ...props }: ButtonProps) {
  return <button data-slot="button" type={type} className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}

export { Button, buttonVariants };
