import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { ListResponse, Zone } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { Waypoints } from "lucide-react";

export function DNSPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const [zones, setZones] = useState<Zone[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .get<ListResponse<Zone>>(`/api/v1/customers/${id}/dns-zones`)
      .then((res) => setZones(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("dns.title")}</h1>
      <PageIntro text={t("dns.description")} />

      {zones.length === 0 ? (
        <EmptyState icon={Waypoints} message={t("dns.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {zones.map((zone) => (
            <Link
              key={zone.id}
              to={`/customers/${id}/dns/${zone.id}`}
              className="block rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200 transition-shadow hover:shadow-md"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">{zone.name}</h3>
                <StatusBadge status={zone.status} />
              </div>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("dns.region")}</dt>
                  <dd className="font-medium text-gray-700">{zone.region_id}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("dns.tenant")}</dt>
                  <dd
                    className="max-w-[180px] truncate font-mono text-xs text-gray-700"
                    title={zone.tenant_id}
                  >
                    {zone.tenant_id}
                  </dd>
                </div>
              </dl>
              {zone.status_message && (
                <p className="mt-3 text-xs text-gray-400">
                  {zone.status_message}
                </p>
              )}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
