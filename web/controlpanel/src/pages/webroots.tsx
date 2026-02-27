import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { ListResponse, Webroot } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { Globe } from "lucide-react";

export function WebrootsPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const [webroots, setWebroots] = useState<Webroot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .get<ListResponse<Webroot>>(`/api/v1/customers/${id}/webroots`)
      .then((res) => setWebroots(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("webroots.title")}</h1>
      <PageIntro text={t("webroots.description")} />

      {webroots.length === 0 ? (
        <EmptyState icon={Globe} message={t("webroots.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {webroots.map((webroot) => (
            <Link
              key={webroot.id}
              to={`/customers/${id}/webroots/${webroot.id}`}
              className="block rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200 transition-shadow hover:shadow-md"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">{webroot.id}</h3>
                <StatusBadge status={webroot.status} />
              </div>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("webroots.runtime")}</dt>
                  <dd className="font-medium text-gray-700">
                    {webroot.runtime} {webroot.runtime_version}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("webroots.publicFolder")}</dt>
                  <dd className="font-mono text-xs text-gray-700">
                    {webroot.public_folder}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("webroots.tenant")}</dt>
                  <dd
                    className="max-w-[180px] truncate font-mono text-xs text-gray-700"
                    title={webroot.tenant_id}
                  >
                    {webroot.tenant_id}
                  </dd>
                </div>
              </dl>
              {webroot.status_message && (
                <p className="mt-3 text-xs text-gray-400">
                  {webroot.status_message}
                </p>
              )}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
