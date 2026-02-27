import { Info } from "lucide-react";

export function FieldHelp({ text }: { text: string }) {
  return (
    <span className="group relative ml-1 inline-block align-middle">
      <Info className="h-3.5 w-3.5 text-gray-400 group-hover:text-gray-600" />
      <span className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1.5 w-56 -translate-x-1/2 rounded-lg bg-gray-900 px-3 py-2 text-xs text-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100">
        {text}
        <span className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900" />
      </span>
    </span>
  );
}
