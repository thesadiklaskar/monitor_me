import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

function Table({ className, ...props }: ComponentProps<"table">) {
  return <div data-slot="table-container" className="relative w-full overflow-x-auto"><table data-slot="table" className={cn("w-full caption-bottom text-left text-xs", className)} {...props} /></div>;
}
function TableHeader({ className, ...props }: ComponentProps<"thead">) {
  return <thead data-slot="table-header" className={cn("[&_tr]:border-b [&_tr]:border-[var(--border)]", className)} {...props} />;
}
function TableBody({ className, ...props }: ComponentProps<"tbody">) {
  return <tbody data-slot="table-body" className={cn("[&_tr:last-child]:border-0", className)} {...props} />;
}
function TableFooter({ className, ...props }: ComponentProps<"tfoot">) {
  return <tfoot data-slot="table-footer" className={cn("border-t border-[var(--border)] bg-[var(--surface-raised)] font-medium", className)} {...props} />;
}
function TableRow({ className, ...props }: ComponentProps<"tr">) {
  return <tr data-slot="table-row" className={cn("border-b border-[var(--border)] transition-colors hover:bg-[rgba(255,255,255,.025)] data-[state=selected]:bg-[var(--surface-raised)]", className)} {...props} />;
}
function TableHead({ className, ...props }: ComponentProps<"th">) {
  return <th data-slot="table-head" className={cn("h-10 px-3 text-left align-middle text-[10px] font-semibold uppercase tracking-wider text-[var(--muted)]", className)} {...props} />;
}
function TableCell({ className, ...props }: ComponentProps<"td">) {
  return <td data-slot="table-cell" className={cn("px-3 py-2.5 align-middle", className)} {...props} />;
}
function TableCaption({ className, ...props }: ComponentProps<"caption">) {
  return <caption data-slot="table-caption" className={cn("mt-3 text-xs text-[var(--muted)]", className)} {...props} />;
}

export { Table, TableHeader, TableBody, TableFooter, TableHead, TableRow, TableCell, TableCaption };
