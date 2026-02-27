import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { ListResponse, Database } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { Database as DatabaseIcon } from "lucide-react";

export function DatabasesPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const [databases, setDatabases] = useState<Database[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .get<ListResponse<Database>>(`/api/v1/customers/${id}/databases`)
      .then((res) => setDatabases(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("databases.title")}</h1>
      <PageIntro text={t("databases.description")} />

      {databases.length === 0 ? (
        <EmptyState icon={DatabaseIcon} message={t("databases.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {databases.map((db) => (
            <Link
              key={db.id}
              to={`/customers/${id}/databases/${db.id}`}
              className="block rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200 transition-shadow hover:shadow-md"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">{db.id}</h3>
                <StatusBadge status={db.status} />
              </div>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("databases.tenant")}</dt>
                  <dd
                    className="max-w-[180px] truncate font-mono text-xs text-gray-700"
                    title={db.tenant_id}
                  >
                    {db.tenant_id}
                  </dd>
                </div>
              </dl>
              {db.status_message && (
                <p className="mt-3 text-xs text-gray-400">
                  {db.status_message}
                </p>
              )}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
