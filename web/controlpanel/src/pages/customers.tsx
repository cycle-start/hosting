import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { Customer, ListResponse } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { getLastCustomerId } from "@/lib/last-customer";
import { Users } from "lucide-react";

export function CustomersPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Skip auto-redirect when the user explicitly navigated here (e.g. "View all customers").
  const showAll = location.state?.showAll === true;

  // If we already know which customer to show, redirect immediately without
  // fetching the full customer list (which could be thousands of rows).
  const lastId = getLastCustomerId();
  useEffect(() => {
    if (!showAll && lastId) {
      navigate(`/customers/${lastId}`, { replace: true });
      return;
    }

    api
      .get<ListResponse<Customer>>("/api/v1/customers")
      .then((res) => setCustomers(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("customers.title")}</h1>
      <PageIntro text={t("customers.description")} />

      {customers.length === 0 ? (
        <EmptyState icon={Users} message={t("customers.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {customers.map((customer) => (
            <Link
              key={customer.id}
              to={`/customers/${customer.id}`}
              className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200 transition-shadow hover:shadow-md"
            >
              <div className="flex items-start justify-between">
                <div className="min-w-0">
                  <h3 className="truncate font-semibold text-gray-900">
                    {customer.name}
                  </h3>
                  {customer.email && (
                    <p className="mt-1 truncate text-sm text-gray-500">
                      {customer.email}
                    </p>
                  )}
                </div>
                <StatusBadge status={customer.status} />
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
