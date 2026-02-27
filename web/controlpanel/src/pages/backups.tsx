import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { ListResponse, Backup } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { Archive } from "lucide-react";
import { ConfirmDialog } from "@/components/confirm-dialog";

function formatBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return `${(bytes / 1_073_741_824).toFixed(1)} GB`;
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

export function BackupsPage() {
  const { id } = useParams<{ id: string }>();
  const { t, i18n } = useTranslation();
  const [backups, setBackups] = useState<Backup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [acting, setActing] = useState<string | null>(null);
  const [confirmAction, setConfirmAction] = useState<{type: "restore" | "delete", backup: Backup} | null>(null);

  function fetchBackups() {
    setLoading(true);
    api
      .get<ListResponse<Backup>>(`/api/v1/customers/${id}/backups`)
      .then((res) => setBackups(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    fetchBackups();
  }, [id]);

  function handleRestore(backupId: string) {
    setActing(backupId);
    api
      .post(`/api/v1/backups/${backupId}/restore`, {})
      .then(() => fetchBackups())
      .catch((err) => setError(err.message))
      .finally(() => setActing(null));
  }

  function handleDelete(backupId: string) {
    setActing(backupId);
    api
      .delete(`/api/v1/backups/${backupId}`)
      .then(() => fetchBackups())
      .catch((err) => setError(err.message))
      .finally(() => setActing(null));
  }

  function handleConfirm() {
    if (!confirmAction) return;
    const { type, backup } = confirmAction;
    setConfirmAction(null);
    if (type === "restore") {
      handleRestore(backup.id);
    } else {
      handleDelete(backup.id);
    }
  }

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("backups.title")}</h1>
      <PageIntro text={t("backups.description")} />

      {backups.length === 0 ? (
        <EmptyState icon={Archive} message={t("backups.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {backups.map((backup) => (
            <div
              key={backup.id}
              className="block rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">
                  {backup.source_name}
                </h3>
                <StatusBadge status={backup.status} />
              </div>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("common.type")}</dt>
                  <dd className="font-medium text-gray-700">{backup.type}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("backups.size")}</dt>
                  <dd className="font-medium text-gray-700">
                    {formatBytes(backup.size_bytes)}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("backups.started")}</dt>
                  <dd className="font-medium text-gray-700">
                    {backup.started_at
                      ? new Date(backup.started_at).toLocaleString(i18n.language)
                      : "—"}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("backups.completed")}</dt>
                  <dd className="font-medium text-gray-700">
                    {backup.completed_at
                      ? new Date(backup.completed_at).toLocaleString(i18n.language)
                      : "—"}
                  </dd>
                </div>
              </dl>
              {backup.status_message && (
                <p className="mt-3 text-xs text-gray-400">
                  {backup.status_message}
                </p>
              )}
              <div className="mt-4 flex justify-end gap-2">
                <button
                  onClick={() => setConfirmAction({ type: "restore", backup })}
                  disabled={acting === backup.id}
                  className="rounded-md bg-brand-50 px-3 py-1.5 text-xs font-medium text-brand-700 ring-1 ring-inset ring-brand-600/20 transition-colors hover:bg-brand-100 disabled:opacity-50"
                >
                  {acting === backup.id ? "..." : t("common.restore")}
                </button>
                <button
                  onClick={() => setConfirmAction({ type: "delete", backup })}
                  disabled={acting === backup.id}
                  className="rounded-md bg-red-50 px-3 py-1.5 text-xs font-medium text-red-700 ring-1 ring-inset ring-red-600/20 transition-colors hover:bg-red-100 disabled:opacity-50"
                >
                  {acting === backup.id ? "..." : t("common.delete")}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
      <ConfirmDialog
        open={confirmAction !== null}
        title={confirmAction?.type === "delete" ? t("backups.deleteTitle") : t("backups.restoreTitle")}
        message={
          confirmAction?.type === "delete"
            ? t("backups.deleteMessage", { name: confirmAction.backup.source_name })
            : t("backups.restoreMessage", { name: confirmAction?.backup.source_name })
        }
        confirmLabel={confirmAction?.type === "delete" ? t("common.delete") : t("common.restore")}
        variant={confirmAction?.type === "delete" ? "danger" : "default"}
        onConfirm={handleConfirm}
        onCancel={() => setConfirmAction(null)}
      />
    </div>
  );
}
