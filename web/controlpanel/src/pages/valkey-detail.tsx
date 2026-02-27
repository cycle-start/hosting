import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { FieldHelp } from "@/components/field-help";
import {
  Check,
  Info,
  Plus,
  Trash2,
  Users as UsersIcon,
  X,
} from "lucide-react";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { DetailPageSkeleton, TabSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { ErrorAlert } from "@/components/error-alert";

type Tab = "users" | "info";

interface ValkeyInstance {
  id: string;
  port: number;
  max_memory_mb: number;
  tenant_id: string;
  subscription_id: string;
  status: string;
  status_message?: string;
  created_at: string;
  updated_at: string;
}

interface ValkeyUser {
  id: string;
  username: string;
  privileges: string[];
  key_pattern: string;
  status: string;
}

interface ListResponse<T> {
  items: T[];
}

export function ValkeyDetailPage() {
  const { id, instanceId } = useParams<{ id: string; instanceId: string }>();
  const { t } = useTranslation();
  const [instance, setInstance] = useState<ValkeyInstance | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState<Tab>("users");

  useEffect(() => {
    api
      .get<ValkeyInstance>(`/api/v1/valkey/${instanceId}`)
      .then(setInstance)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [instanceId]);

  if (loading) return <DetailPageSkeleton tabs={2} />;

  if (error) return <ErrorPage error={error} />;

  if (!instance) return null;

  return (
    <div className="p-8">
      <Breadcrumbs
        items={[
          { label: t("nav.valkey"), href: `/customers/${id}/valkey` },
          { label: instance.id },
        ]}
      />

      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{instance.id}</h1>
          <p className="mt-1 text-sm text-gray-500">
            {t("valkeyDetail.port")}: {instance.port} &middot;{" "}
            {t("valkeyDetail.maxMemory")}: {instance.max_memory_mb} MB
          </p>
        </div>
        <StatusBadge status={instance.status} />
      </div>

      {instance.status_message && (
        <div className="mb-6 rounded-lg bg-yellow-50 px-4 py-3 text-sm text-yellow-700">
          {instance.status_message}
        </div>
      )}

      <div className="mb-6 flex gap-1 rounded-lg bg-gray-100 p-1">
        <TabButton
          active={activeTab === "users"}
          onClick={() => setActiveTab("users")}
          icon={UsersIcon}
          label={t("valkeyDetail.users.tab")}
        />
        <TabButton
          active={activeTab === "info"}
          onClick={() => setActiveTab("info")}
          icon={Info}
          label={t("valkeyDetail.info.tab")}
        />
      </div>

      {activeTab === "users" && <UsersTab instanceId={instanceId!} />}
      {activeTab === "info" && <InfoTab instance={instance} />}
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

function UsersTab({ instanceId }: { instanceId: string }) {
  const { t } = useTranslation();
  const [users, setUsers] = useState<ValkeyUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ValkeyUser | null>(null);

  const fetchUsers = () => {
    setLoading(true);
    api
      .get<ListResponse<ValkeyUser>>(`/api/v1/valkey/${instanceId}/users`)
      .then((res) => {
        setUsers(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchUsers();
  }, [instanceId]);

  const handleCreate = async (
    username: string,
    password: string,
    privileges: string[],
    keyPattern: string,
  ) => {
    setSaving(true);
    try {
      await api.post(`/api/v1/valkey/${instanceId}/users`, {
        username,
        password,
        privileges,
        key_pattern: keyPattern,
      });
      setShowForm(false);
      fetchUsers();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (userId: string) => {
    setDeleting(userId);
    try {
      await api.delete(`/api/v1/valkey/${instanceId}/users/${userId}`);
      fetchUsers();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setDeleting(null);
    }
  };

  const confirmDelete = () => {
    if (!deleteTarget) return;
    handleDelete(deleteTarget.id);
    setDeleteTarget(null);
  };

  if (loading) {
    return (
      <TabSkeleton />
    );
  }

  return (
    <div>
      {error && <ErrorAlert message={error} />}

      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">
          {t("valkeyDetail.users.title")}
        </h2>
        {!showForm && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("valkeyDetail.users.addUser")}
          </button>
        )}
      </div>

      {showForm && (
        <ValkeyUserForm
          onSave={handleCreate}
          onCancel={() => setShowForm(false)}
          saving={saving}
        />
      )}

      {users.length === 0 && !showForm ? (
        <EmptyState icon={UsersIcon} message={t("valkeyDetail.users.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("valkeyDetail.users.username")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("valkeyDetail.users.privileges")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("valkeyDetail.users.keyPattern")}
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
              {users.map((u) => (
                <tr key={u.id}>
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-sm text-gray-900">
                    {u.username}
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500">
                    {u.privileges.join(", ")}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-sm text-gray-500">
                    {u.key_pattern}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    <StatusBadge status={u.status} />
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                    <button
                      onClick={() => setDeleteTarget(u)}
                      disabled={deleting === u.id}
                      className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                      title={t("common.delete")}
                    >
                      {deleting === u.id ? (
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
        title={t("valkeyDetail.users.deleteTitle")}
        message={t("valkeyDetail.users.deleteMessage", {
          name: deleteTarget?.username,
        })}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

function ValkeyUserForm({
  onSave,
  onCancel,
  saving,
}: {
  onSave: (
    username: string,
    password: string,
    privileges: string[],
    keyPattern: string,
  ) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const { t } = useTranslation();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [privileges, setPrivileges] = useState("");
  const [keyPattern, setKeyPattern] = useState("*");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!username || !password) return;
    const privs = privileges
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    onSave(username, password, privs, keyPattern);
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      <div className="mb-3 grid gap-3 sm:grid-cols-2">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("valkeyDetail.users.username")}
          </label>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="valkey_user"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("valkeyDetail.users.password")}
          </label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Password"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
      </div>
      <div className="mb-3 grid gap-3 sm:grid-cols-2">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("valkeyDetail.users.privileges")}
          </label>
          <input
            type="text"
            value={privileges}
            onChange={(e) => setPrivileges(e.target.value)}
            placeholder="+@all, +get, +set"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("valkeyDetail.users.keyPattern")}
            <FieldHelp text={t("valkeyDetail.users.keyPatternHelp")} />
          </label>
          <input
            type="text"
            value={keyPattern}
            onChange={(e) => setKeyPattern(e.target.value)}
            placeholder="*"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
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
            disabled={saving || !username || !password}
            className="inline-flex items-center gap-1 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
          >
            <Check className="h-4 w-4" />
            {saving ? t("common.saving") : t("common.add")}
          </button>
        </div>
      </div>
    </form>
  );
}

function InfoTab({ instance }: { instance: ValkeyInstance }) {
  const { t, i18n } = useTranslation();

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        {t("valkeyDetail.info.title")}
      </h2>
      <div className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
        <dl className="grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("common.status")}
            </dt>
            <dd className="mt-1 text-sm text-gray-900">
              <StatusBadge status={instance.status} />
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("valkeyDetail.port")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {instance.port}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("valkeyDetail.maxMemory")}
            </dt>
            <dd className="mt-1 text-sm text-gray-900">
              {instance.max_memory_mb} MB
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("common.tenantId")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {instance.tenant_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("common.subscriptionId")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {instance.subscription_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("common.createdAt")}
            </dt>
            <dd className="mt-1 text-sm text-gray-900">
              {new Date(instance.created_at).toLocaleString(i18n.language)}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("common.updatedAt")}
            </dt>
            <dd className="mt-1 text-sm text-gray-900">
              {new Date(instance.updated_at).toLocaleString(i18n.language)}
            </dd>
          </div>
        </dl>
      </div>
    </div>
  );
}
