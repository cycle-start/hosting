import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { ListResponse, S3Bucket } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { HardDrive } from "lucide-react";

function formatBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return `${(bytes / 1_073_741_824).toFixed(1)} GB`;
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

export function S3BucketsPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const [buckets, setBuckets] = useState<S3Bucket[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .get<ListResponse<S3Bucket>>(`/api/v1/customers/${id}/s3-buckets`)
      .then((res) => setBuckets(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("s3Buckets.title")}</h1>
      <PageIntro text={t("s3Buckets.description")} />

      {buckets.length === 0 ? (
        <EmptyState icon={HardDrive} message={t("s3Buckets.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {buckets.map((bucket) => (
            <Link
              key={bucket.id}
              to={`/customers/${id}/s3/${bucket.id}`}
              className="block rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200 transition-shadow hover:shadow-md"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">{bucket.id}</h3>
                <StatusBadge status={bucket.status} />
              </div>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("s3Buckets.access")}</dt>
                  <dd className="font-medium text-gray-700">
                    {bucket.public ? t("common.public") : t("common.private")}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("s3Buckets.quota")}</dt>
                  <dd className="font-medium text-gray-700">
                    {formatBytes(bucket.quota_bytes)}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("s3Buckets.tenant")}</dt>
                  <dd
                    className="max-w-[180px] truncate font-mono text-xs text-gray-700"
                    title={bucket.tenant_id}
                  >
                    {bucket.tenant_id}
                  </dd>
                </div>
              </dl>
              {bucket.status_message && (
                <p className="mt-3 text-xs text-gray-400">
                  {bucket.status_message}
                </p>
              )}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
