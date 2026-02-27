import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { ConfirmDialog } from "@/components/confirm-dialog";
import {
  AlertTriangle,
  Check,
  Info,
  Key,
  Plus,
  Settings,
  Trash2,
  X,
} from "lucide-react";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { DetailPageSkeleton, TabSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { ErrorAlert } from "@/components/error-alert";

type Tab = "access-keys" | "settings" | "info";

interface S3Bucket {
  id: string;
  public: boolean;
  quota_bytes: number;
  tenant_id: string;
  subscription_id: string;
  status: string;
  status_message?: string;
  created_at: string;
  updated_at: string;
}

interface S3AccessKey {
  id: string;
  access_key_id: string;
  secret_access_key?: string;
  permissions: string[];
  status: string;
}

interface ListResponse<T> {
  items: T[];
}

const ALL_PERMISSIONS = ["read", "write", "list", "delete"];

export function S3BucketDetailPage() {
  const { id, bucketId } = useParams<{ id: string; bucketId: string }>();
  const { t } = useTranslation();
  const [bucket, setBucket] = useState<S3Bucket | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState<Tab>("access-keys");

  const fetchBucket = () => {
    api
      .get<S3Bucket>(`/api/v1/s3-buckets/${bucketId}`)
      .then(setBucket)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchBucket();
  }, [bucketId]);

  if (loading) return <DetailPageSkeleton tabs={3} />;

  if (error) return <ErrorPage error={error} />;

  if (!bucket) return null;

  const quotaMB = bucket.quota_bytes
    ? Math.round(bucket.quota_bytes / 1024 / 1024)
    : 0;

  return (
    <div className="p-8">
      <Breadcrumbs
        items={[
          { label: t("nav.s3Storage"), href: `/customers/${id}/s3` },
          { label: bucket.id },
        ]}
      />

      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{bucket.id}</h1>
          <p className="mt-1 text-sm text-gray-500">
            {bucket.public ? t("common.public") : t("common.private")} &middot;{" "}
            {quotaMB > 0 ? t("s3BucketDetail.quotaLabel", { value: quotaMB }) : t("s3BucketDetail.noQuota")}
          </p>
        </div>
        <StatusBadge status={bucket.status} />
      </div>

      {bucket.status_message && (
        <div className="mb-6 rounded-lg bg-yellow-50 px-4 py-3 text-sm text-yellow-700">
          {bucket.status_message}
        </div>
      )}

      <div className="mb-6 flex gap-1 rounded-lg bg-gray-100 p-1">
        <TabButton
          active={activeTab === "access-keys"}
          onClick={() => setActiveTab("access-keys")}
          icon={Key}
          label={t("s3BucketDetail.accessKeys.tab")}
        />
        <TabButton
          active={activeTab === "settings"}
          onClick={() => setActiveTab("settings")}
          icon={Settings}
          label={t("s3BucketDetail.settings.tab")}
        />
        <TabButton
          active={activeTab === "info"}
          onClick={() => setActiveTab("info")}
          icon={Info}
          label={t("s3BucketDetail.info.tab")}
        />
      </div>

      {activeTab === "access-keys" && (
        <AccessKeysTab bucketId={bucketId!} />
      )}
      {activeTab === "settings" && (
        <SettingsTab bucket={bucket} onSaved={fetchBucket} />
      )}
      {activeTab === "info" && <InfoTab bucket={bucket} />}
    </div>
  );
}

function TabButton({
  active,
  onClick,
  icon: Icon,
  label,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-2 rounded-md px-4 py-2 text-sm font-medium transition-colors ${
        active
          ? "bg-white text-gray-900 shadow-sm"
          : "text-gray-500 hover:text-gray-700"
      }`}
    >
      <Icon className="h-4 w-4" />
      {label}
    </button>
  );
}

function AccessKeysTab({ bucketId }: { bucketId: string }) {
  const { t } = useTranslation();
  const [keys, setKeys] = useState<S3AccessKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<S3AccessKey | null>(null);
  const [newSecret, setNewSecret] = useState<string | null>(null);

  const fetchKeys = () => {
    setLoading(true);
    api
      .get<ListResponse<S3AccessKey>>(
        `/api/v1/s3-buckets/${bucketId}/access-keys`,
      )
      .then((res) => {
        setKeys(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchKeys();
  }, [bucketId]);

  const handleCreate = async (permissions: string[]) => {
    setSaving(true);
    try {
      const res = await api.post<S3AccessKey>(
        `/api/v1/s3-buckets/${bucketId}/access-keys`,
        { permissions },
      );
      setShowForm(false);
      if (res.secret_access_key) {
        setNewSecret(res.secret_access_key);
      }
      fetchKeys();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (keyId: string) => {
    setDeleting(keyId);
    try {
      await api.delete(`/api/v1/s3-buckets/${bucketId}/access-keys/${keyId}`);
      fetchKeys();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setDeleting(null);
    }
  };

  const confirmDelete = () => {
    if (deleteTarget) {
      handleDelete(deleteTarget.id);
      setDeleteTarget(null);
    }
  };

  if (loading) {
    return (
      <TabSkeleton />
    );
  }

  return (
    <div>
      {error && <ErrorAlert message={error} />}

      {newSecret && (
        <div className="mb-4 rounded-xl bg-amber-50 p-4 shadow-sm ring-1 ring-amber-200">
          <div className="mb-2 flex items-center gap-2 text-sm font-medium text-amber-800">
            <AlertTriangle className="h-4 w-4" />
            {t("s3BucketDetail.accessKeys.secretWarning")}
          </div>
          <div className="flex items-center gap-2">
            <code className="flex-1 rounded-lg bg-white px-3 py-2 font-mono text-sm text-gray-900 ring-1 ring-amber-200">
              {newSecret}
            </code>
            <button
              onClick={() => setNewSecret(null)}
              className="rounded-lg px-3 py-2 text-sm text-amber-700 hover:bg-amber-100"
            >
              {t("common.dismiss")}
            </button>
          </div>
        </div>
      )}

      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">{t("s3BucketDetail.accessKeys.title")}</h2>
        {!showForm && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("s3BucketDetail.accessKeys.createAccessKey")}
          </button>
        )}
      </div>

      {showForm && (
        <AccessKeyForm
          onSave={handleCreate}
          onCancel={() => setShowForm(false)}
          saving={saving}
        />
      )}

      {keys.length === 0 && !showForm ? (
        <EmptyState icon={Key} message={t("s3BucketDetail.accessKeys.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("s3BucketDetail.accessKeys.accessKeyId")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("s3BucketDetail.accessKeys.permissions")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.status")}
                </th>
                <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.actions")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {keys.map((k) => (
                <tr key={k.id}>
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-sm text-gray-900">
                    {k.access_key_id}
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500">
                    {k.permissions.join(", ")}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    <StatusBadge status={k.status} />
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                    <button
                      onClick={() => setDeleteTarget(k)}
                      disabled={deleting === k.id}
                      className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                      title={t("common.delete")}
                    >
                      {deleting === k.id ? (
                        <span className="text-xs">...</span>
                      ) : (
                        <Trash2 className="inline h-4 w-4" />
                      )}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t("s3BucketDetail.accessKeys.deleteTitle")}
        message={t("s3BucketDetail.accessKeys.deleteMessage", { name: deleteTarget?.access_key_id })}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

function AccessKeyForm({
  onSave,
  onCancel,
  saving,
}: {
  onSave: (permissions: string[]) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const { t } = useTranslation();
  const [permissions, setPermissions] = useState<string[]>([]);

  const togglePermission = (perm: string) => {
    setPermissions((prev) =>
      prev.includes(perm) ? prev.filter((p) => p !== perm) : [...prev, perm],
    );
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (permissions.length === 0) return;
    onSave(permissions);
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      <div className="mb-3">
        <label className="mb-1 block text-xs font-medium text-gray-700">
          {t("s3BucketDetail.accessKeys.permissions")}
        </label>
        <div className="flex flex-wrap gap-3">
          {ALL_PERMISSIONS.map((perm) => (
            <label
              key={perm}
              className="flex items-center gap-1.5 text-sm text-gray-700"
            >
              <input
                type="checkbox"
                checked={permissions.includes(perm)}
                onChange={() => togglePermission(perm)}
                className="rounded border-gray-300 text-brand-600 focus:ring-brand-500"
              />
              {perm}
            </label>
          ))}
        </div>
      </div>
      <div className="flex items-center justify-end">
        <div className="flex gap-2">
          <button
            type="button"
            onClick={onCancel}
            className="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-100"
          >
            <X className="h-4 w-4" />
            {t("common.cancel")}
          </button>
          <button
            type="submit"
            disabled={saving || permissions.length === 0}
            className="inline-flex items-center gap-1 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
          >
            <Check className="h-4 w-4" />
            {saving ? t("common.creating") : t("common.create")}
          </button>
        </div>
      </div>
    </form>
  );
}

function SettingsTab({
  bucket,
  onSaved,
}: {
  bucket: S3Bucket;
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const [isPublic, setIsPublic] = useState(bucket.public);
  const [quotaMB, setQuotaMB] = useState(
    bucket.quota_bytes ? Math.round(bucket.quota_bytes / 1024 / 1024) : 0,
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    setError("");
    setSuccess(false);
    try {
      await api.put(`/api/v1/s3-buckets/${bucket.id}`, {
        public: isPublic,
        quota_bytes: quotaMB > 0 ? quotaMB * 1024 * 1024 : 0,
      });
      setSuccess(true);
      onSaved();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        {t("s3BucketDetail.settings.title")}
      </h2>

      {error && <ErrorAlert message={error} />}

      {success && (
        <div className="mb-4 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">
          {t("s3BucketDetail.settings.saved")}
        </div>
      )}

      <div className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
        <div className="mb-4">
          <label className="flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={isPublic}
              onChange={(e) => setIsPublic(e.target.checked)}
              className="rounded border-gray-300 text-brand-600 focus:ring-brand-500"
            />
            {t("s3BucketDetail.settings.publicAccess")}
          </label>
          <p className="mt-1 text-xs text-gray-500">
            {t("s3BucketDetail.settings.publicDescription")}
          </p>
        </div>

        <div className="mb-4">
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("s3BucketDetail.settings.quotaMB")}
          </label>
          <input
            type="number"
            value={quotaMB}
            onChange={(e) => setQuotaMB(Number(e.target.value))}
            min={0}
            placeholder="0 = unlimited"
            className="w-full max-w-xs rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
          <p className="mt-1 text-xs text-gray-500">
            {t("s3BucketDetail.settings.quotaUnlimited")}
          </p>
        </div>

        <button
          onClick={handleSave}
          disabled={saving}
          className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
        >
          <Check className="h-4 w-4" />
          {saving ? t("common.saving") : t("s3BucketDetail.settings.saveSettings")}
        </button>
      </div>
    </div>
  );
}

function InfoTab({ bucket }: { bucket: S3Bucket }) {
  const { t, i18n } = useTranslation();

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        {t("s3BucketDetail.info.title")}
      </h2>
      <div className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
        <dl className="grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("common.status")}</dt>
            <dd className="mt-1 text-sm text-gray-900">
              <StatusBadge status={bucket.status} />
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("common.tenantId")}</dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {bucket.tenant_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("common.subscriptionId")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {bucket.subscription_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("common.createdAt")}</dt>
            <dd className="mt-1 text-sm text-gray-900">
              {new Date(bucket.created_at).toLocaleString(i18n.language)}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("common.updatedAt")}</dt>
            <dd className="mt-1 text-sm text-gray-900">
              {new Date(bucket.updated_at).toLocaleString(i18n.language)}
            </dd>
          </div>
        </dl>
      </div>
    </div>
  );
}
