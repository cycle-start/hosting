import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type { WireGuardPeer } from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { DetailPageSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { Info, FileText, Trash2, Copy } from "lucide-react";

type Tab = "info" | "config";

export function WireGuardDetailPage() {
  const { id, peerId } = useParams<{ id: string; peerId: string }>();
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [peer, setPeer] = useState<WireGuardPeer | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState<Tab>("info");
  const [showDelete, setShowDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    api
      .get<WireGuardPeer>(`/api/v1/wireguard/${peerId}`)
      .then(setPeer)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [peerId]);

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await api.delete(`/api/v1/wireguard/${peerId}`);
      navigate(`/customers/${id}/wireguard`);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setDeleting(false);
      setShowDelete(false);
    }
  };

  if (loading) return <DetailPageSkeleton tabs={2} />;

  if (error) return <ErrorPage error={error} />;

  if (!peer) return null;

  return (
    <div className="p-8">
      <Breadcrumbs
        items={[
          { label: t("nav.wireguard"), href: `/customers/${id}/wireguard` },
          { label: peer.name },
        ]}
      />

      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{peer.name}</h1>
          <p className="mt-1 text-sm text-gray-500">
            {peer.assigned_ip} &middot; {peer.endpoint}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <StatusBadge status={peer.status} />
          <button
            onClick={() => setShowDelete(true)}
            disabled={deleting}
            className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
          >
            <Trash2 className="h-4 w-4" />
            {t("common.delete")}
          </button>
        </div>
      </div>

      {peer.status_message && (
        <div className="mb-6 rounded-lg bg-yellow-50 px-4 py-3 text-sm text-yellow-700">
          {peer.status_message}
        </div>
      )}

      <div className="mb-6 flex gap-1 rounded-lg bg-gray-100 p-1">
        <TabButton
          active={activeTab === "info"}
          onClick={() => setActiveTab("info")}
          icon={Info}
          label={t("wireguardDetail.tabs.info")}
        />
        <TabButton
          active={activeTab === "config"}
          onClick={() => setActiveTab("config")}
          icon={FileText}
          label={t("wireguardDetail.tabs.config")}
        />
      </div>

      {activeTab === "info" && <InfoTab peer={peer} />}
      {activeTab === "config" && <ConfigTab peer={peer} />}

      <ConfirmDialog
        open={showDelete}
        title={t("wireguardDetail.delete.title")}
        message={t("wireguardDetail.delete.confirm")}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={handleDelete}
        onCancel={() => setShowDelete(false)}
      />
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

function InfoTab({ peer }: { peer: WireGuardPeer }) {
  const { t, i18n } = useTranslation();

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        {t("wireguardDetail.title")}
      </h2>
      <div className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
        <dl className="grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.name")}
            </dt>
            <dd className="mt-1 text-sm text-gray-900">{peer.name}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("common.status")}
            </dt>
            <dd className="mt-1 text-sm text-gray-900">
              <StatusBadge status={peer.status} />
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.publicKey")}
            </dt>
            <dd className="mt-1 break-all font-mono text-sm text-gray-900">
              {peer.public_key}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.assignedIp")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {peer.assigned_ip}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.endpoint")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {peer.endpoint}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.peerIndex")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {peer.peer_index}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.tenant")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {peer.tenant_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.subscription")}
            </dt>
            <dd className="mt-1 font-mono text-sm text-gray-900">
              {peer.subscription_id}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.createdAt")}
            </dt>
            <dd className="mt-1 text-sm text-gray-900">
              {new Date(peer.created_at).toLocaleString(i18n.language)}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500">
              {t("wireguardDetail.info.updatedAt")}
            </dt>
            <dd className="mt-1 text-sm text-gray-900">
              {new Date(peer.updated_at).toLocaleString(i18n.language)}
            </dd>
          </div>
        </dl>
      </div>
    </div>
  );
}

function ConfigTab({ peer }: { peer: WireGuardPeer }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const configTemplate = `[Interface]
PrivateKey = <your-private-key>
Address = ${peer.assigned_ip}/128

[Peer]
PublicKey = ${peer.endpoint ? "..." : "GATEWAY_PUBLIC_KEY_NOT_CONFIGURED"}
Endpoint = ${peer.endpoint || "<not configured>"}
AllowedIPs = fd00::/16
PersistentKeepalive = 25`;

  const handleCopy = async () => {
    await navigator.clipboard.writeText(configTemplate);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        {t("wireguardDetail.tabs.config")}
      </h2>
      <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200">
        <div className="mb-4 rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-700">
          {t("wireguard.configWarning")}
        </div>
        <pre className="mb-3 overflow-x-auto rounded-lg bg-gray-900 p-4 text-xs text-gray-100">
          {configTemplate}
        </pre>
        <button
          onClick={handleCopy}
          className="inline-flex items-center gap-1.5 rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-gray-700 shadow-sm ring-1 ring-gray-300 hover:bg-gray-50"
        >
          <Copy className="h-4 w-4" />
          {copied ? t("common.copied") : t("wireguard.copyConfig")}
        </button>
      </div>
    </div>
  );
}
