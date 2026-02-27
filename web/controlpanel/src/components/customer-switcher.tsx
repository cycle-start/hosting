import { useEffect, useRef, useState } from "react";
import { useNavigate, Link } from "react-router";
import { useTranslation } from "react-i18next";
import { ChevronDown, Search } from "lucide-react";
import { api } from "@/api/client";
import type { Customer, ListResponse } from "@/api/types";

export function CustomerSwitcher({
  currentCustomerId,
  currentCustomerName,
}: {
  currentCustomerId: string | undefined;
  currentCustomerName: string;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [customers, setCustomers] = useState<Customer[] | null>(null);
  const [search, setSearch] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    if (customers === null) {
      api
        .get<ListResponse<Customer>>("/api/v1/customers")
        .then((res) => setCustomers(res.items))
        .catch(() => setCustomers([]));
    }
    // Focus the search input when opening
    setTimeout(() => searchRef.current?.focus(), 0);
  }, [open]);

  // Close on click outside
  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [open]);

  const filtered = customers?.filter((c) =>
    c.name.toLowerCase().includes(search.toLowerCase()),
  );

  function selectCustomer(id: string) {
    setOpen(false);
    setSearch("");
    navigate(`/customers/${id}`);
  }

  return (
    <div className="relative" ref={containerRef}>
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
      >
        <span className="max-w-[200px] truncate">
          {currentCustomerName || t("topBar.selectCustomer")}
        </span>
        <ChevronDown className="h-4 w-4 text-gray-400" />
      </button>

      {open && (
        <div className="absolute left-0 top-full z-50 mt-1 w-80 rounded-lg bg-white shadow-lg ring-1 ring-gray-200">
          {/* Search */}
          <div className="border-b border-gray-100 p-2">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
              <input
                ref={searchRef}
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t("topBar.searchCustomers")}
                className="w-full rounded-md border border-gray-200 py-1.5 pl-8 pr-3 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500/20 focus:outline-none"
              />
            </div>
          </div>

          {/* Customer list */}
          <div className="max-h-64 overflow-y-auto p-1">
            {filtered === undefined || filtered === null ? (
              <div className="flex items-center justify-center py-4">
                <div className="h-5 w-5 animate-spin rounded-full border-2 border-brand-600 border-t-transparent" />
              </div>
            ) : filtered.length === 0 ? (
              <p className="py-3 text-center text-sm text-gray-500">
                {t("topBar.noCustomersFound")}
              </p>
            ) : (
              filtered.map((customer) => (
                <button
                  key={customer.id}
                  onClick={() => selectCustomer(customer.id)}
                  className={`flex w-full items-center rounded-md px-3 py-2 text-left text-sm transition-colors ${
                    customer.id === currentCustomerId
                      ? "bg-brand-50 font-medium text-brand-700"
                      : "text-gray-700 hover:bg-gray-50"
                  }`}
                >
                  <span className="truncate">{customer.name}</span>
                </button>
              ))
            )}
          </div>

          {/* View all link */}
          <div className="border-t border-gray-100 p-2">
            <Link
              to="/customers"
              state={{ showAll: true }}
              onClick={() => setOpen(false)}
              className="block rounded-md px-3 py-1.5 text-center text-sm font-medium text-brand-600 transition-colors hover:bg-brand-50"
            >
              {t("topBar.viewAllCustomers")}
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
