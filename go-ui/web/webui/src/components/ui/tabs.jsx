import { cn } from "../../lib/utils";

export function Tabs({ className, ...props }) {
  return <div className={cn("inline-flex rounded-lg border border-zinc-800 bg-zinc-900 p-1", className)} {...props} />;
}

export function TabsButton({ active, className, ...props }) {
  return (
    <button
      type="button"
      className={cn(
        "rounded-md px-3 py-1.5 text-sm transition",
        active ? "bg-blue-600 text-white" : "text-zinc-300 hover:bg-zinc-800",
        className,
      )}
      {...props}
    />
  );
}
