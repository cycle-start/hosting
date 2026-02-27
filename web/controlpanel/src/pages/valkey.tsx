import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { ListResponse, ValkeyInstance } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { Zap } from "lucide-react";

export function ValkeyPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const [instances, setInstances] = useState<ValkeyInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .get<ListResponse<ValkeyInstance>>(`/api/v1/customers/${id}/valkey`)
      .then((res) => setInstances(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("valkey.title")}</h1>
      <PageIntro text={t("valkey.description")} />

      {instances.length === 0 ? (
        <EmptyState icon={Zap} message={t("valkey.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {instances.map((instance) => (
            <Link
              key={instance.id}
              to={`/customers/${id}/valkey/${instance.id}`}
              className="block rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200 transition-shadow hover:shadow-md"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">{instance.id}</h3>
                <StatusBadge status={instance.status} />
              </div>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("valkey.port")}</dt>
                  <dd className="font-medium text-gray-700">{instance.port}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("valkey.maxMemory")}</dt>
                  <dd className="font-medium text-gray-700">
                    {instance.max_memory_mb} MB
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("valkey.tenant")}</dt>
                  <dd
                    className="max-w-[180px] truncate font-mono text-xs text-gray-700"
                    title={instance.tenant_id}
                  >
                    {instance.tenant_id}
                  </dd>
                </div>
              </dl>
              {instance.status_message && (
                <p className="mt-3 text-xs text-gray-400">
                  {instance.status_message}
                </p>
              )}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
