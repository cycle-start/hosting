const colors: Record<string, string> = {
  active: "bg-green-50 text-green-700 ring-green-600/20",
  running: "bg-green-50 text-green-700 ring-green-600/20",
  healthy: "bg-green-50 text-green-700 ring-green-600/20",
  inactive: "bg-gray-50 text-gray-600 ring-gray-500/20",
  suspended: "bg-yellow-50 text-yellow-700 ring-yellow-600/20",
  pending: "bg-yellow-50 text-yellow-700 ring-yellow-600/20",
  error: "bg-red-50 text-red-700 ring-red-600/20",
  stopped: "bg-red-50 text-red-700 ring-red-600/20",
};

const fallback = "bg-gray-50 text-gray-600 ring-gray-500/20";

export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${colors[status] || fallback}`}
    >
      {status}
    </span>
  );
}
