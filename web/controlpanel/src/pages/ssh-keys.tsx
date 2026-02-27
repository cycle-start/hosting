import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { ListResponse, SSHKey } from "@/api/types";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { Key } from "lucide-react";

export function SSHKeysPage() {
  const { id } = useParams<{ id: string }>();
  const { t, i18n } = useTranslation();
  const [keys, setKeys] = useState<SSHKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SSHKey | null>(null);

  function fetchKeys() {
    setLoading(true);
    api
      .get<ListResponse<SSHKey>>(`/api/v1/customers/${id}/ssh-keys`)
      .then((res) => setKeys(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    fetchKeys();
  }, [id]);

  function confirmDelete() {
    if (!deleteTarget) return;
    setDeleting(deleteTarget.id);
    setDeleteTarget(null);
    api
      .delete(`/api/v1/ssh-keys/${deleteTarget.id}`)
      .then(() => fetchKeys())
      .catch((err) => setError(err.message))
      .finally(() => setDeleting(null));
  }

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">{t("sshKeys.title")}</h1>
      <PageIntro text={t("sshKeys.description")} />

      {keys.length === 0 ? (
        <EmptyState icon={Key} message={t("sshKeys.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {keys.map((key) => (
            <div
              key={key.id}
              className="block rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">{key.name}</h3>
                <StatusBadge status={key.status} />
              </div>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("sshKeys.fingerprint")}</dt>
                  <dd
                    className="max-w-[180px] truncate font-mono text-xs text-gray-700"
                    title={key.fingerprint}
                  >
                    {key.fingerprint}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("common.created")}</dt>
                  <dd className="font-medium text-gray-700">
                    {new Date(key.created_at).toLocaleDateString(i18n.language)}
                  </dd>
                </div>
              </dl>
              {key.status_message && (
                <p className="mt-3 text-xs text-gray-400">
                  {key.status_message}
                </p>
              )}
              <div className="mt-4 flex justify-end">
                <button
                  onClick={() => setDeleteTarget(key)}
                  disabled={deleting === key.id}
                  className="rounded-md bg-red-50 px-3 py-1.5 text-xs font-medium text-red-700 ring-1 ring-inset ring-red-600/20 transition-colors hover:bg-red-100 disabled:opacity-50"
                >
                  {deleting === key.id ? t("common.deleting") : t("common.delete")}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t("sshKeys.deleteTitle")}
        message={t("sshKeys.deleteMessage", { name: deleteTarget?.name })}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
