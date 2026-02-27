import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type {
  DashboardData,
  ListResponse,
  WireGuardPeer,
  WireGuardPeerCreateResult,
} from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { PageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { ErrorAlert } from "@/components/error-alert";
import {
  ShieldCheck,
  Plus,
  X,
  Check,
  AlertTriangle,
  Download,
  Copy,
} from "lucide-react";

export function WireGuardPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const [peers, setPeers] = useState<WireGuardPeer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [createResult, setCreateResult] =
    useState<WireGuardPeerCreateResult | null>(null);

  const fetchPeers = () => {
    setLoading(true);
    api
      .get<ListResponse<WireGuardPeer>>(`/api/v1/customers/${id}/wireguard`)
      .then((res) => setPeers(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchPeers();
  }, [id]);

  const handleCreated = (result: WireGuardPeerCreateResult) => {
    setCreateResult(result);
    setShowForm(false);
    fetchPeers();
  };

  if (loading) return <PageSkeleton />;

  if (error) return <ErrorPage error={error} />;

  return (
    <div className="p-8">
      <div className="mb-2 flex items-start justify-between">
        <h1 className="text-2xl font-bold text-gray-900">
          {t("wireguard.title")}
        </h1>
        {!showForm && !createResult && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("wireguard.create")}
          </button>
        )}
      </div>
      <PageIntro text={t("wireguard.description")} />

      {createResult && (
        <ConfigDisplay
          result={createResult}
          onDismiss={() => setCreateResult(null)}
        />
      )}

      {showForm && (
        <CreatePeerForm
          customerId={id!}
          onCreated={handleCreated}
          onCancel={() => setShowForm(false)}
        />
      )}

      {peers.length === 0 && !showForm ? (
        <EmptyState icon={ShieldCheck} message={t("wireguard.empty")} />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {peers.map((peer) => (
            <Link
              key={peer.id}
              to={`/customers/${id}/wireguard/${peer.id}`}
              className="block rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200 transition-shadow hover:shadow-md"
            >
              <div className="mb-3 flex items-start justify-between">
                <h3 className="font-semibold text-gray-900">{peer.name}</h3>
                <StatusBadge status={peer.status} />
              </div>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("wireguard.assignedIp")}</dt>
                  <dd className="font-mono text-gray-700">
                    {peer.assigned_ip}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-gray-500">{t("wireguard.endpoint")}</dt>
                  <dd className="font-mono text-gray-700">{peer.endpoint}</dd>
                </div>
              </dl>
              {peer.status_message && (
                <p className="mt-3 text-xs text-gray-400">
                  {peer.status_message}
                </p>
              )}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function CreatePeerForm({
  customerId,
  onCreated,
  onCancel,
}: {
  customerId: string;
  onCreated: (result: WireGuardPeerCreateResult) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [subscriptionId, setSubscriptionId] = useState<string | null>(null);
  const [loadingSubs, setLoadingSubs] = useState(true);

  useEffect(() => {
    api
      .get<DashboardData>(`/api/v1/customers/${customerId}/dashboard`)
      .then((data) => {
        const wgSub = data.subscriptions.find((s) =>
          s.modules.includes("wireguard"),
        );
        if (wgSub) {
          setSubscriptionId(wgSub.id);
        } else if (data.subscriptions.length > 0) {
          setSubscriptionId(data.subscriptions[0].id);
        }
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoadingSubs(false));
  }, [customerId]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name || !subscriptionId) return;
    setSaving(true);
    setError("");
    try {
      const result = await api.post<WireGuardPeerCreateResult>(
        `/api/v1/customers/${customerId}/wireguard`,
        { name, subscription_id: subscriptionId },
      );
      onCreated(result);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      {error && <ErrorAlert message={error} />}
      <div className="mb-3">
        <label className="mb-1 block text-xs font-medium text-gray-700">
          {t("wireguard.name")}
        </label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("wireguard.namePlaceholder")}
          className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
        />
      </div>
      <div className="flex items-center justify-end gap-2">
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
          disabled={saving || !name || loadingSubs || !subscriptionId}
          className="inline-flex items-center gap-1 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
        >
          <Check className="h-4 w-4" />
          {saving ? t("common.creating") : t("common.create")}
        </button>
      </div>
    </form>
  );
}

function ConfigDisplay({
  result,
  onDismiss,
}: {
  result: WireGuardPeerCreateResult;
  onDismiss: () => void;
}) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(result.client_config);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleDownload = () => {
    const blob = new Blob([result.client_config], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${result.peer.name}.conf`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="mb-6 rounded-xl bg-amber-50 p-4 shadow-sm ring-1 ring-amber-200">
      <div className="mb-3 flex items-start gap-2">
        <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" />
        <p className="text-sm font-medium text-amber-800">
          {t("wireguard.configWarning")}
        </p>
      </div>
      <pre className="mb-3 overflow-x-auto rounded-lg bg-gray-900 p-4 text-xs text-gray-100">
        {result.client_config}
      </pre>
      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={handleDownload}
          className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
        >
          <Download className="h-4 w-4" />
          {t("wireguard.downloadConfig")}
        </button>
        <button
          onClick={handleCopy}
          className="inline-flex items-center gap-1.5 rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-gray-700 shadow-sm ring-1 ring-gray-300 hover:bg-gray-50"
        >
          <Copy className="h-4 w-4" />
          {copied ? t("common.copied") : t("wireguard.copyConfig")}
        </button>
        <button
          onClick={onDismiss}
          className="ml-auto inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-gray-600 hover:bg-amber-100"
        >
          <X className="h-4 w-4" />
          {t("common.dismiss")}
        </button>
      </div>
      <p className="mt-3 font-mono text-xs text-amber-700">{t("wireguard.cliHint")}</p>
    </div>
  );
}
