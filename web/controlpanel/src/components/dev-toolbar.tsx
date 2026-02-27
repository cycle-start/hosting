import { useState } from "react";
import { ArrowRightLeft } from "lucide-react";
import { getDevPartnerOverride, setDevPartnerOverride } from "@/api/client";

export function DevToolbar() {
  const [hostname, setHostname] = useState(getDevPartnerOverride() ?? "");
  const [open, setOpen] = useState(false);

  const active = getDevPartnerOverride();

  function apply() {
    const trimmed = hostname.trim();
    setDevPartnerOverride(trimmed || null);
    window.location.reload();
  }

  function clear() {
    setDevPartnerOverride(null);
    setHostname("");
    window.location.reload();
  }

  return (
    <div className="fixed bottom-3 right-3 z-50">
      {open ? (
        <div className="rounded-lg bg-gray-900 p-3 text-sm text-white shadow-xl ring-1 ring-white/10">
          <div className="mb-2 flex items-center justify-between gap-4">
            <span className="font-medium">Partner Override</span>
            <button
              onClick={() => setOpen(false)}
              className="text-gray-400 hover:text-white"
            >
              &times;
            </button>
          </div>
          <div className="flex gap-2">
            <input
              type="text"
              value={hostname}
              onChange={(e) => setHostname(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && apply()}
              placeholder="e.g. bobs.localhost"
              className="w-48 rounded border border-gray-700 bg-gray-800 px-2 py-1 text-sm text-white placeholder-gray-500 focus:border-brand-500 focus:outline-none"
            />
            <button
              onClick={apply}
              className="rounded bg-brand-600 px-2 py-1 text-xs font-medium hover:bg-brand-700"
            >
              Apply
            </button>
          </div>
          {active && (
            <button
              onClick={clear}
              className="mt-2 text-xs text-gray-400 hover:text-white"
            >
              Clear override (back to default)
            </button>
          )}
        </div>
      ) : (
        <button
          onClick={() => setOpen(true)}
          className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium shadow-lg ${
            active
              ? "bg-amber-500 text-white"
              : "bg-gray-800 text-gray-300 hover:text-white"
          }`}
          title={active ? `Override: ${active}` : "Switch partner"}
        >
          <ArrowRightLeft className="h-3 w-3" />
          {active ? active : "DEV"}
        </button>
      )}
    </div>
  );
}
