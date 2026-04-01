import { cn } from "../../lib/utils";

export function Badge({ className, ...props }) {
  return (
    <span
      className={cn("inline-flex rounded-full px-2.5 py-1 text-xs font-medium", className)}
      {...props}
    />
  );
}
