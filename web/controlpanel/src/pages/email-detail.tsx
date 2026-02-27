import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { ConfirmDialog } from "@/components/confirm-dialog";
import {
  Check,
  Forward,
  Info,
  Mail,
  MessageSquare,
  Plus,
  Trash2,
  X,
} from "lucide-react";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { DetailPageSkeleton, TabSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { ErrorAlert } from "@/components/error-alert";

type Tab = "aliases" | "forwards" | "autoreply" | "info";

interface EmailAccount {
  id: string;
  address: string;
  display_name: string;
  quota: number;
  fqdn_id: string;
  tenant_id: string;
  subscription_id: string;
  status: string;
  status_message?: string;
  created_at: string;
  updated_at: string;
}

interface EmailAlias {
  id: string;
  address: string;
  status: string;
}

interface EmailForward {
  id: string;
  destination: string;
  keep_copy: boolean;
  status: string;
}

interface EmailAutoreply {
  id: string;
  subject: string;
  body: string;
  start_date: string;
  end_date: string;
  enabled: boolean;
}

interface ListResponse<T> {
  items: T[];
}

export function EmailDetailPage() {
  const { id, accountId } = useParams<{ id: string; accountId: string }>();
  const { t } = useTranslation();
  const [account, setAccount] = useState<EmailAccount | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState<Tab>("aliases");

  useEffect(() => {
    api
      .get<EmailAccount>(`/api/v1/email/${accountId}`)
      .then(setAccount)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [accountId]);

  if (loading) return <DetailPageSkeleton tabs={4} />;

  if (error) return <ErrorPage error={error} />;

  if (!account) return null;

  return (
    <div className="p-8">
      <Breadcrumbs
        items={[
          { label: t("nav.email"), href: `/customers/${id}/email` },
          { label: account.address },
        ]}
      />

      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">
            {account.address}
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            {account.display_name} &middot; {t("emailDetail.quota")}: {account.quota} MB
          </p>
        </div>
        <StatusBadge status={account.status} />
      </div>

      {account.status_message && (
        <div className="mb-6 rounded-lg bg-yellow-50 px-4 py-3 text-sm text-yellow-700">
          {account.status_message}
        </div>
      )}

      <div className="mb-6 flex gap-1 rounded-lg bg-gray-100 p-1">
        <TabButton
          active={activeTab === "aliases"}
          onClick={() => setActiveTab("aliases")}
          icon={Mail}
          label={t("emailDetail.aliases.tab")}
        />
        <TabButton
          active={activeTab === "forwards"}
          onClick={() => setActiveTab("forwards")}
          icon={Forward}
          label={t("emailDetail.forwards.tab")}
        />
        <TabButton
          active={activeTab === "autoreply"}
          onClick={() => setActiveTab("autoreply")}
          icon={MessageSquare}
          label={t("emailDetail.autoreply.tab")}
        />
        <TabButton
          active={activeTab === "info"}
          onClick={() => setActiveTab("info")}
          icon={Info}
          label={t("emailDetail.info.tab")}
        />
      </div>

      {activeTab === "aliases" && <AliasesTab accountId={accountId!} />}
      {activeTab === "forwards" && <ForwardsTab accountId={accountId!} />}
      {activeTab === "autoreply" && <AutoreplyTab accountId={accountId!} />}
      {activeTab === "info" && <InfoTab account={account} />}
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

function AliasesTab({ accountId }: { accountId: string }) {
  const { t } = useTranslation();
  const [aliases, setAliases] = useState<EmailAlias[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<EmailAlias | null>(null);

  const fetchAliases = () => {
    setLoading(true);
    api
      .get<ListResponse<EmailAlias>>(`/api/v1/email/${accountId}/aliases`)
      .then((res) => {
        setAliases(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchAliases();
  }, [accountId]);

  const handleCreate = async (address: string) => {
    setSaving(true);
    try {
      await api.post(`/api/v1/email/${accountId}/aliases`, { address });
      setShowForm(false);
      fetchAliases();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (aliasId: string) => {
    setDeleting(aliasId);
    try {
      await api.delete(`/api/v1/email/${accountId}/aliases/${aliasId}`);
      fetchAliases();
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
        <h2 className="text-lg font-semibold text-gray-900">{t("emailDetail.aliases.title")}</h2>
        {!showForm && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("emailDetail.aliases.addAlias")}
          </button>
        )}
      </div>

      {showForm && (
        <AliasForm
          onSave={handleCreate}
          onCancel={() => setShowForm(false)}
          saving={saving}
        />
      )}

      {aliases.length === 0 && !showForm ? (
        <EmptyState icon={Mail} message={t("emailDetail.aliases.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("emailDetail.aliases.address")}
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
              {aliases.map((a) => (
                <tr key={a.id}>
                  <td className="whitespace-nowrap px-4 py-3 text-sm text-gray-900">
                    {a.address}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    <StatusBadge status={a.status} />
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                    <button
                      onClick={() => setDeleteTarget(a)}
                      disabled={deleting === a.id}
                      className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                      title={t("common.delete")}
                    >
                      {deleting === a.id ? (
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
        title={t("emailDetail.aliases.deleteTitle")}
        message={t("emailDetail.aliases.deleteMessage", { name: deleteTarget?.address })}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

function AliasForm({
  onSave,
  onCancel,
  saving,
}: {
  onSave: (address: string) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const { t } = useTranslation();
  const [address, setAddress] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!address) return;
    onSave(address);
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      <div className="mb-3">
        <label className="mb-1 block text-xs font-medium text-gray-700">
          {t("emailDetail.aliases.address")}
        </label>
        <input
          type="email"
          value={address}
          onChange={(e) => setAddress(e.target.value)}
          placeholder="alias@example.com"
          className="w-full max-w-md rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
        />
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
            disabled={saving || !address}
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

function ForwardsTab({ accountId }: { accountId: string }) {
  const { t } = useTranslation();
  const [forwards, setForwards] = useState<EmailForward[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<EmailForward | null>(null);

  const fetchForwards = () => {
    setLoading(true);
    api
      .get<ListResponse<EmailForward>>(`/api/v1/email/${accountId}/forwards`)
      .then((res) => {
        setForwards(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchForwards();
  }, [accountId]);

  const handleCreate = async (destination: string, keepCopy: boolean) => {
    setSaving(true);
    try {
      await api.post(`/api/v1/email/${accountId}/forwards`, {
        destination,
        keep_copy: keepCopy,
      });
      setShowForm(false);
      fetchForwards();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (forwardId: string) => {
    setDeleting(forwardId);
    try {
      await api.delete(`/api/v1/email/${accountId}/forwards/${forwardId}`);
      fetchForwards();
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
        <h2 className="text-lg font-semibold text-gray-900">{t("emailDetail.forwards.title")}</h2>
        {!showForm && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("emailDetail.forwards.addForward")}
          </button>
        )}
      </div>

      {showForm && (
        <ForwardForm
          onSave={handleCreate}
          onCancel={() => setShowForm(false)}
          saving={saving}
        />
      )}

      {forwards.length === 0 && !showForm ? (
        <EmptyState icon={Forward} message={t("emailDetail.forwards.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("emailDetail.forwards.destination")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("emailDetail.forwards.keepCopy")}
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
              {forwards.map((f) => (
                <tr key={f.id}>
                  <td className="whitespace-nowrap px-4 py-3 text-sm text-gray-900">
                    {f.destination}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm text-gray-500">
                    {f.keep_copy ? t("common.yes") : t("common.no")}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    <StatusBadge status={f.status} />
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                    <button
                      onClick={() => setDeleteTarget(f)}
                      disabled={deleting === f.id}
                      className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                      title={t("common.delete")}
                    >
                      {deleting === f.id ? (
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
        title={t("emailDetail.forwards.deleteTitle")}
        message={t("emailDetail.forwards.deleteMessage", { name: deleteTarget?.destination })}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

function ForwardForm({
  onSave,
  onCancel,
  saving,
}: {
  onSave: (destination: string, keepCopy: boolean) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const { t } = useTranslation();
  const [destination, setDestination] = useState("");
  const [keepCopy, setKeepCopy] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!destination) return;
    onSave(destination, keepCopy);
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      <div className="mb-3">
        <label className="mb-1 block text-xs font-medium text-gray-700">
          {t("emailDetail.forwards.destination")}
        </label>
        <input
          type="email"
          value={destination}
          onChange={(e) => setDestination(e.target.value)}
          placeholder="forward@example.com"
          className="w-full max-w-md rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
        />
      </div>
      <div className="mb-3">
        <label className="flex items-center gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            checked={keepCopy}
            onChange={(e) => setKeepCopy(e.target.checked)}
            className="rounded border-gray-300 text-brand-600 focus:ring-brand-500"
          />
          {t("emailDetail.forwards.keepCopyLabel")}
        </label>
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
            disabled={saving || !destination}
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

function AutoreplyTab({ accountId }: { accountId: string }) {
  const { t } = useTranslation();
  const [autoreply, setAutoreply] = useState<EmailAutoreply | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [enabled, setEnabled] = useState(false);

  const fetchAutoreply = () => {
    setLoading(true);
    api
      .get<EmailAutoreply>(`/api/v1/email/${accountId}/autoreply`)
      .then((res) => {
        setAutoreply(res);
        setSubject(res.subject);
        setBody(res.body);
        setStartDate(res.start_date ? res.start_date.split("T")[0] : "");
        setEndDate(res.end_date ? res.end_date.split("T")[0] : "");
        setEnabled(res.enabled);
        setError("");
      })
      .catch((err) => {
        if (err.status === 404) {
          setAutoreply(null);
          setError("");
        } else {
          setError(err.message);
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchAutoreply();
  }, [accountId]);

  const handleSave = async () => {
    setSaving(true);
    setError("");
    setSuccess(false);
    try {
      await api.put(`/api/v1/email/${accountId}/autoreply`, {
        subject,
        body,
        start_date: startDate,
        end_date: endDate,
        enabled,
      });
      setSuccess(true);
      fetchAutoreply();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    setDeleteConfirmOpen(false);
    try {
      await api.delete(`/api/v1/email/${accountId}/autoreply`);
      setAutoreply(null);
      setSubject("");
      setBody("");
      setStartDate("");
      setEndDate("");
      setEnabled(false);
    } catch (err: any) {
      setError(err.message);
    }
  };

  if (loading) {
    return (
      <TabSkeleton />
    );
  }

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">{t("emailDetail.autoreply.title")}</h2>

      {error && <ErrorAlert message={error} />}

      {success && (
        <div className="mb-4 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">
          {t("emailDetail.autoreply.saved")}
        </div>
      )}

      <div className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
        <div className="mb-3">
          <label className="flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="rounded border-gray-300 text-brand-600 focus:ring-brand-500"
            />
            {t("common.enabled")}
          </label>
        </div>

        <div className="mb-3">
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("emailDetail.autoreply.subject")}
          </label>
          <input
            type="text"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            placeholder="Out of office"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>

        <div className="mb-3">
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("emailDetail.autoreply.body")}
          </label>
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={4}
            placeholder="I am currently out of the office..."
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>

        <div className="mb-3 grid gap-3 sm:grid-cols-2">
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-700">
              {t("emailDetail.autoreply.startDate")}
            </label>
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-700">
              {t("emailDetail.autoreply.endDate")}
            </label>
            <input
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
            />
          </div>
        </div>

        <div className="flex items-center justify-between">
          {autoreply && (
            <button
              onClick={() => setDeleteConfirmOpen(true)}
              className="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-red-600 hover:bg-red-50"
            >
              <Trash2 className="h-4 w-4" />
              {t("emailDetail.autoreply.deleteAutoreply")}
            </button>
          )}
          <div className={`flex gap-2 ${!autoreply ? "ml-auto" : ""}`}>
            <button
              onClick={handleSave}
              disabled={saving}
              className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
            >
              <Check className="h-4 w-4" />
              {saving ? t("common.saving") : t("common.save")}
            </button>
          </div>
        </div>
      </div>

      <ConfirmDialog
        open={deleteConfirmOpen}
        title={t("emailDetail.autoreply.deleteTitle")}
        message={t("emailDetail.autoreply.deleteMessage")}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={handleDelete}
        onCancel={() => setDeleteConfirmOpen(false)}
      />
    </div>
  );
}

function InfoTab({ account }: { account: EmailAccount }) {
  const { t, i18n } = useTranslation();
  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        {t("emailDetail.info.title")}
      </h2>
      <div className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
        <dl className="grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("common.status")}</dt>
            <dd className="mt-1 text-sm text-gray-900">
              <StatusBadge status={account.status} />
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("emailDetail.info.displayName")}</dt>
            <dd className="mt-1 text-sm text-gray-900">
              {account.display_name}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("emailDetail.info.quota")}</dt>
            <dd className="mt-1 text-sm text-gray-900">{account.quota} MB</dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("emailDetail.info.fqdnId")}</dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {account.fqdn_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("common.tenantId")}</dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {account.tenant_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("common.subscriptionId")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {account.subscription_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("common.createdAt")}</dt>
            <dd className="mt-1 text-sm text-gray-900">
              {new Date(account.created_at).toLocaleString(i18n.language)}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">{t("common.updatedAt")}</dt>
            <dd className="mt-1 text-sm text-gray-900">
              {new Date(account.updated_at).toLocaleString(i18n.language)}
            </dd>
          </div>
        </dl>
      </div>
    </div>
  );
}
