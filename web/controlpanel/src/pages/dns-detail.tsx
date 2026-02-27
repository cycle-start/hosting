import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { FieldHelp } from "@/components/field-help";
import {
  Check,
  Edit2,
  Globe,
  Plus,
  Trash2,
  X,
} from "lucide-react";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { DetailPageSkeleton, TabSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { ErrorAlert } from "@/components/error-alert";

interface Zone {
  id: string;
  name: string;
  region_id: string;
  tenant_id: string;
  subscription_id: string;
  status: string;
  status_message?: string;
  created_at: string;
  updated_at: string;
}

interface ZoneRecord {
  id: string;
  type: string;
  name: string;
  content: string;
  ttl: number;
  priority?: number;
  managed_by: string;
  status: string;
}

interface ListResponse<T> {
  items: T[];
}

const RECORD_TYPES = ["A", "AAAA", "CNAME", "MX", "TXT", "SRV", "NS", "CAA"];

export function DNSDetailPage() {
  const { id, zoneId } = useParams<{ id: string; zoneId: string }>();
  const { t } = useTranslation();
  const [zone, setZone] = useState<Zone | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .get<Zone>(`/api/v1/dns-zones/${zoneId}`)
      .then(setZone)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [zoneId]);

  if (loading) return <DetailPageSkeleton tabs={0} />;

  if (error) return <ErrorPage error={error} />;

  if (!zone) return null;

  return (
    <div className="p-8">
      <Breadcrumbs
        items={[
          { label: t("dns.title"), href: `/customers/${id}/dns` },
          { label: zone.name },
        ]}
      />

      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{zone.name}</h1>
          <p className="mt-1 text-sm text-gray-500">
            {t("dnsDetail.region")}: {zone.region_id}
          </p>
        </div>
        <StatusBadge status={zone.status} />
      </div>

      {zone.status_message && (
        <div className="mb-6 rounded-lg bg-yellow-50 px-4 py-3 text-sm text-yellow-700">
          {zone.status_message}
        </div>
      )}

      <RecordsView zoneId={zoneId!} />
    </div>
  );
}

function RecordsView({ zoneId }: { zoneId: string }) {
  const { t } = useTranslation();
  const [records, setRecords] = useState<ZoneRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ZoneRecord | null>(null);

  const fetchRecords = () => {
    setLoading(true);
    api
      .get<ListResponse<ZoneRecord>>(`/api/v1/dns-zones/${zoneId}/records`)
      .then((res) => {
        setRecords(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchRecords();
  }, [zoneId]);

  const handleCreate = async (data: {
    type: string;
    name: string;
    content: string;
    ttl: number;
    priority?: number;
  }) => {
    setSaving(true);
    try {
      await api.post(`/api/v1/dns-zones/${zoneId}/records`, data);
      setShowForm(false);
      fetchRecords();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleUpdate = async (
    recordId: string,
    data: {
      type: string;
      name: string;
      content: string;
      ttl: number;
      priority?: number;
    },
  ) => {
    setSaving(true);
    try {
      await api.put(`/api/v1/dns-zones/${zoneId}/records/${recordId}`, data);
      setEditingId(null);
      fetchRecords();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (recordId: string) => {
    setDeleting(recordId);
    try {
      await api.delete(`/api/v1/dns-zones/${zoneId}/records/${recordId}`);
      fetchRecords();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setDeleting(null);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    await handleDelete(deleteTarget.id);
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
        <h2 className="text-lg font-semibold text-gray-900">{t("dnsDetail.records.title")}</h2>
        {!showForm && editingId === null && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("dnsDetail.records.addRecord")}
          </button>
        )}
      </div>

      {showForm && (
        <RecordForm
          onSave={handleCreate}
          onCancel={() => setShowForm(false)}
          saving={saving}
        />
      )}

      {records.length === 0 && !showForm ? (
        <EmptyState icon={Globe} message={t("dnsDetail.records.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("dnsDetail.records.recordType")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.name")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("dnsDetail.records.content")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("dnsDetail.records.ttl")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("dnsDetail.records.priority")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("dnsDetail.records.managedBy")}
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
              {records.map((r) =>
                editingId === r.id ? (
                  <InlineEditRow
                    key={r.id}
                    record={r}
                    onSave={(data) => handleUpdate(r.id, data)}
                    onCancel={() => setEditingId(null)}
                    saving={saving}
                  />
                ) : (
                  <tr key={r.id}>
                    <td className="whitespace-nowrap px-4 py-3 text-sm">
                      <span className="inline-flex items-center rounded-md bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 ring-1 ring-inset ring-blue-600/20">
                        {r.type}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 font-mono text-sm text-gray-900">
                      {r.name}
                    </td>
                    <td className="max-w-xs truncate px-4 py-3 font-mono text-sm text-gray-500" title={r.content}>
                      {r.content}
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 text-sm text-gray-500">
                      {r.ttl}
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 text-sm text-gray-500">
                      {r.priority != null ? r.priority : "-"}
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 text-sm text-gray-500">
                      {r.managed_by}
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 text-sm">
                      <StatusBadge status={r.status} />
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                      {r.managed_by === "custom" ? (
                        <div className="inline-flex gap-2">
                          <button
                            onClick={() => setEditingId(r.id)}
                            className="text-gray-400 hover:text-brand-600"
                            title={t("common.edit")}
                          >
                            <Edit2 className="inline h-4 w-4" />
                          </button>
                          <button
                            onClick={() => setDeleteTarget(r)}
                            disabled={deleting === r.id}
                            className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                            title={t("common.delete")}
                          >
                            {deleting === r.id ? (
                              <span className="text-xs">...</span>
                            ) : (
                              <Trash2 className="inline h-4 w-4" />
                            )}
                          </button>
                        </div>
                      ) : (
                        <span className="text-gray-300">-</span>
                      )}
                    </td>
                  </tr>
                ),
              )}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t("dnsDetail.records.deleteTitle")}
        message={
          deleteTarget
            ? t("dnsDetail.records.deleteMessage", { type: deleteTarget.type, name: deleteTarget.name })
            : ""
        }
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

function RecordForm({
  onSave,
  onCancel,
  saving,
  initial,
}: {
  onSave: (data: {
    type: string;
    name: string;
    content: string;
    ttl: number;
    priority?: number;
  }) => void;
  onCancel: () => void;
  saving: boolean;
  initial?: ZoneRecord;
}) {
  const { t } = useTranslation();
  const [type, setType] = useState(initial?.type || "A");
  const [name, setName] = useState(initial?.name || "");
  const [content, setContent] = useState(initial?.content || "");
  const [ttl, setTtl] = useState(initial?.ttl?.toString() || "3600");
  const [priority, setPriority] = useState(
    initial?.priority?.toString() || "",
  );

  const showPriority = type === "MX" || type === "SRV";

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name || !content) return;
    const data: {
      type: string;
      name: string;
      content: string;
      ttl: number;
      priority?: number;
    } = {
      type,
      name,
      content,
      ttl: Number(ttl) || 3600,
    };
    if (showPriority && priority) {
      data.priority = Number(priority);
    }
    onSave(data);
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      <div className="mb-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("dnsDetail.records.recordType")}
          </label>
          <select
            value={type}
            onChange={(e) => setType(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          >
            {RECORD_TYPES.map((rt) => (
              <option key={rt} value={rt}>
                {rt}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("common.name")}
          </label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="@"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("dnsDetail.records.content")}
          </label>
          <input
            type="text"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder="192.168.1.1"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("dnsDetail.records.ttl")}
            <FieldHelp text={t("dnsDetail.records.ttlHelp")} />
          </label>
          <input
            type="number"
            value={ttl}
            onChange={(e) => setTtl(e.target.value)}
            placeholder="3600"
            min={60}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
      </div>

      {showPriority && (
        <div className="mb-3">
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("dnsDetail.records.priority")}
            <FieldHelp text={t("dnsDetail.records.priorityHelp")} />
          </label>
          <input
            type="number"
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
            placeholder="10"
            min={0}
            className="w-full max-w-xs rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
      )}

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
            disabled={saving || !name || !content}
            className="inline-flex items-center gap-1 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
          >
            <Check className="h-4 w-4" />
            {saving ? t("common.saving") : initial ? t("common.update") : t("common.add")}
          </button>
        </div>
      </div>
    </form>
  );
}

function InlineEditRow({
  record,
  onSave,
  onCancel,
  saving,
}: {
  record: ZoneRecord;
  onSave: (data: {
    type: string;
    name: string;
    content: string;
    ttl: number;
    priority?: number;
  }) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const { t } = useTranslation();
  const [type, setType] = useState(record.type);
  const [name, setName] = useState(record.name);
  const [content, setContent] = useState(record.content);
  const [ttl, setTtl] = useState(record.ttl.toString());
  const [priority, setPriority] = useState(
    record.priority?.toString() || "",
  );

  const showPriority = type === "MX" || type === "SRV";

  const handleSave = () => {
    const data: {
      type: string;
      name: string;
      content: string;
      ttl: number;
      priority?: number;
    } = {
      type,
      name,
      content,
      ttl: Number(ttl) || 3600,
    };
    if (showPriority && priority) {
      data.priority = Number(priority);
    }
    onSave(data);
  };

  return (
    <tr className="bg-brand-50">
      <td className="whitespace-nowrap px-4 py-2">
        <select
          value={type}
          onChange={(e) => setType(e.target.value)}
          className="w-full rounded border border-gray-300 px-2 py-1 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
        >
          {RECORD_TYPES.map((rt) => (
            <option key={rt} value={rt}>
              {rt}
            </option>
          ))}
        </select>
      </td>
      <td className="whitespace-nowrap px-4 py-2">
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full rounded border border-gray-300 px-2 py-1 font-mono text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
        />
      </td>
      <td className="px-4 py-2">
        <input
          type="text"
          value={content}
          onChange={(e) => setContent(e.target.value)}
          className="w-full rounded border border-gray-300 px-2 py-1 font-mono text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
        />
      </td>
      <td className="whitespace-nowrap px-4 py-2">
        <input
          type="number"
          value={ttl}
          onChange={(e) => setTtl(e.target.value)}
          min={60}
          className="w-20 rounded border border-gray-300 px-2 py-1 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
        />
      </td>
      <td className="whitespace-nowrap px-4 py-2">
        {showPriority ? (
          <input
            type="number"
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
            min={0}
            className="w-16 rounded border border-gray-300 px-2 py-1 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        ) : (
          <span className="text-sm text-gray-400">-</span>
        )}
      </td>
      <td className="whitespace-nowrap px-4 py-2 text-sm text-gray-500">
        {record.managed_by}
      </td>
      <td className="whitespace-nowrap px-4 py-2 text-sm">
        <StatusBadge status={record.status} />
      </td>
      <td className="whitespace-nowrap px-4 py-2 text-right text-sm">
        <div className="inline-flex gap-2">
          <button
            onClick={handleSave}
            disabled={saving || !name || !content}
            className="text-brand-600 hover:text-brand-700 disabled:opacity-50"
            title={t("common.save")}
          >
            {saving ? (
              <span className="text-xs">...</span>
            ) : (
              <Check className="inline h-4 w-4" />
            )}
          </button>
          <button
            onClick={onCancel}
            className="text-gray-400 hover:text-gray-600"
            title={t("common.cancel")}
          >
            <X className="inline h-4 w-4" />
          </button>
        </div>
      </td>
    </tr>
  );
}
