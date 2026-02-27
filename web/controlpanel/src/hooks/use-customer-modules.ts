import { createContext, useContext, useEffect, useState } from "react";
import { api } from "@/api/client";
import type { DashboardData } from "@/api/types";
import React from "react";
import { useAuth } from "@/auth/context";

interface CustomerModulesContextValue {
  modules: string[];
  customerName: string;
  loading: boolean;
}

const CustomerModulesContext = createContext<CustomerModulesContextValue>({
  modules: [],
  customerName: "",
  loading: true,
});

export function CustomerModulesProvider({
  customerId,
  children,
}: {
  customerId: string | undefined;
  children: React.ReactNode;
}) {
  const [modules, setModules] = useState<string[]>([]);
  const [customerName, setCustomerName] = useState("");
  const [loading, setLoading] = useState(true);
  const { updateUser } = useAuth();

  useEffect(() => {
    if (!customerId) {
      setModules([]);
      setCustomerName("");
      setLoading(false);
      return;
    }

    setLoading(true);
    api
      .get<DashboardData>(`/api/v1/customers/${customerId}/dashboard`)
      .then((data) => {
        setModules(data.enabled_modules);
        setCustomerName(data.customer_name);
        // Persist last visited customer
        updateUser({ last_customer_id: customerId });
        api.patch("/api/v1/me", { last_customer_id: customerId }).catch(() => {});
      })
      .catch(() => {
        setModules([]);
        setCustomerName("");
      })
      .finally(() => setLoading(false));
  }, [customerId]);

  return React.createElement(
    CustomerModulesContext.Provider,
    { value: { modules, customerName, loading } },
    children,
  );
}

export function useCustomerModules() {
  return useContext(CustomerModulesContext);
}
