import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { DashboardData } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { MODULE_META } from "@/lib/modules";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { Package } from "lucide-react";

export function DashboardPage() {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .get<DashboardData>(`/api/v1/customers/${id}/dashboard`)
      .then(setData)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  if (!data) return null;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("dashboard.title")}</h1>
      <PageIntro text={t("dashboard.description")} />

      {data.subscriptions.length === 0 ? (
        <EmptyState icon={Package} message={t("dashboard.emptySubscriptions")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {data.subscriptions.map((sub) => (
            <div
              key={sub.id}
              className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">
                  {sub.product_name}
                </h3>
                <StatusBadge status={sub.status} />
              </div>
              {sub.product_description && (
                <p className="mb-3 text-sm text-gray-500">
                  {sub.product_description}
                </p>
              )}
              {sub.modules.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {sub.modules.map((mod) => {
                    const meta = MODULE_META[mod];
                    if (meta) {
                      const Icon = meta.icon;
                      return (
                        <Link
                          key={mod}
                          to={`/customers/${id}/${meta.route}`}
                          className="inline-flex items-center gap-1.5 rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 transition-colors hover:bg-brand-50 hover:text-brand-700"
                        >
                          <Icon className="h-3 w-3" />
                          {t(meta.labelKey)}
                        </Link>
                      );
                    }
                    return (
                      <span
                        key={mod}
                        className="rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600"
                      >
                        {mod}
                      </span>
                    );
                  })}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
