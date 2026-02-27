import type { LucideIcon } from "lucide-react";

export function EmptyState({
  icon: Icon,
  message,
}: {
  icon: LucideIcon;
  message: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-gray-300 py-12 text-center">
      <Icon className="mb-3 h-10 w-10 text-gray-400" />
      <p className="text-sm text-gray-500">{message}</p>
    </div>
  );
}
