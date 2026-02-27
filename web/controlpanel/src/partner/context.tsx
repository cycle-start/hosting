import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { api } from "@/api/client";
import type { Partner } from "@/api/types";

interface PartnerState {
  partner: Partner | null;
  isLoading: boolean;
  error: string | null;
}

const PartnerContext = createContext<PartnerState>({
  partner: null,
  isLoading: true,
  error: null,
});

export function PartnerProvider({ children }: { children: ReactNode }) {
  const [partner, setPartner] = useState<Partner | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<Partner>("/partner")
      .then((p) => {
        setPartner(p);
        // Apply partner's brand color as CSS custom property (OKLCH hue angle)
        document.documentElement.style.setProperty(
          "--brand-hue",
          p.primary_color,
        );
      })
      .catch((err) => setError(err.message))
      .finally(() => setIsLoading(false));
  }, []);

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-gray-300 border-t-transparent" />
      </div>
    );
  }

  if (error || !partner) {
    return (
      <div className="flex h-screen items-center justify-center p-8">
        <div className="rounded-lg bg-red-50 px-6 py-4 text-sm text-red-700">
          {error || "Unable to load partner configuration"}
        </div>
      </div>
    );
  }

  return (
    <PartnerContext.Provider value={{ partner, isLoading, error }}>
      {children}
    </PartnerContext.Provider>
  );
}

export function usePartner() {
  return useContext(PartnerContext);
}
