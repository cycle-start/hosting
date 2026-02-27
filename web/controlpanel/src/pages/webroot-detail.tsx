import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import type {
  CronJob,
  Daemon,
  EnvVar,
  FQDN,
  ListResponse,
  RuntimeGroup,
  VaultEncryptResponse,
  Webroot,
} from "@/api/types";
import { StatusBadge } from "@/components/status-badge";
import { EmptyState } from "@/components/empty-state";
import { PageIntro } from "@/components/page-intro";
import { FieldHelp } from "@/components/field-help";
import {
  Check,
  Clock,
  ClipboardCopy,
  Globe,
  Lock,
  Pencil,
  Play,
  Plus,
  Settings,
  TerminalSquare,
  Trash2,
  Unlink,
  Wrench,
  X,
} from "lucide-react";
import { WebTerminal } from "@/components/web-terminal";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { DetailPageSkeleton, TabSkeleton } from "@/components/skeleton";
import { ErrorPage } from "@/components/error-page";
import { ErrorAlert } from "@/components/error-alert";

type Tab = "env-vars" | "vault" | "daemons" | "cron-jobs" | "fqdns" | "settings" | "terminal";

const ENV_NAME_RE = /^[A-Za-z_][A-Za-z0-9_]{0,127}$/;

export function WebrootDetailPage() {
  const { id, webrootId } = useParams<{ id: string; webrootId: string }>();
  const { t } = useTranslation();
  const [webroot, setWebroot] = useState<Webroot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState<Tab>("env-vars");

  useEffect(() => {
    api
      .get<Webroot>(`/api/v1/webroots/${webrootId}`)
      .then(setWebroot)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [webrootId]);

  if (loading) return <DetailPageSkeleton tabs={7} />;

  if (error) return <ErrorPage error={error} />;

  if (!webroot) return null;

  return (
    <div className="p-8">
      <Breadcrumbs
        items={[
          { label: t("webroots.title"), href: `/customers/${id}/webroots` },
          { label: webroot.id },
        ]}
      />

      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{webroot.id}</h1>
          <p className="mt-1 text-sm text-gray-500">
            {webroot.runtime} {webroot.runtime_version} &middot;{" "}
            {webroot.public_folder}
          </p>
        </div>
        <StatusBadge status={webroot.status} />
      </div>

      {webroot.status_message && (
        <div className="mb-6 rounded-lg bg-yellow-50 px-4 py-3 text-sm text-yellow-700">
          {webroot.status_message}
        </div>
      )}

      <div className="mb-6 flex flex-wrap gap-1 rounded-lg bg-gray-100 p-1">
        <TabButton
          active={activeTab === "env-vars"}
          onClick={() => setActiveTab("env-vars")}
          icon={Settings}
          label={t("webrootDetail.envVars.tab")}
        />
        <TabButton
          active={activeTab === "vault"}
          onClick={() => setActiveTab("vault")}
          icon={Lock}
          label={t("webrootDetail.vault.tab")}
        />
        <TabButton
          active={activeTab === "daemons"}
          onClick={() => setActiveTab("daemons")}
          icon={Play}
          label={t("webrootDetail.daemons.tab")}
        />
        <TabButton
          active={activeTab === "cron-jobs"}
          onClick={() => setActiveTab("cron-jobs")}
          icon={Clock}
          label={t("webrootDetail.cronJobs.tab")}
        />
        <TabButton
          active={activeTab === "fqdns"}
          onClick={() => setActiveTab("fqdns")}
          icon={Globe}
          label={t("webrootDetail.fqdns.tab")}
        />
        <TabButton
          active={activeTab === "settings"}
          onClick={() => setActiveTab("settings")}
          icon={Wrench}
          label={t("webrootDetail.settings.tab")}
        />
        <TabButton
          active={activeTab === "terminal"}
          onClick={() => setActiveTab("terminal")}
          icon={TerminalSquare}
          label={t("webrootDetail.terminal.tab")}
        />
      </div>

      {activeTab === "env-vars" && <EnvVarsTab webrootId={webrootId!} />}
      {activeTab === "vault" && <VaultTab webrootId={webrootId!} />}
      {activeTab === "daemons" && <DaemonsTab webrootId={webrootId!} />}
      {activeTab === "cron-jobs" && <CronJobsTab webrootId={webrootId!} />}
      {activeTab === "fqdns" && <FQDNsTab webrootId={webrootId!} />}
      {activeTab === "settings" && (
        <SettingsTab webrootId={webrootId!} webroot={webroot} onUpdate={setWebroot} />
      )}
      {activeTab === "terminal" && (
        <div>
          <h2 className="mb-2 text-lg font-semibold text-gray-900">
            {t("webrootDetail.terminal.title")}
          </h2>
          <p className="mb-4 text-sm text-gray-500">
            {t("webrootDetail.terminal.description")}
          </p>
          <WebTerminal webrootId={webrootId!} />
        </div>
      )}
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

// ─── Daemons Tab ────────────────────────────────────────────────────────────

function DaemonsTab({ webrootId }: { webrootId: string }) {
  const { t } = useTranslation();
  const [daemons, setDaemons] = useState<Daemon[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [editingDaemon, setEditingDaemon] = useState<Daemon | null>(null);
  const [saving, setSaving] = useState(false);
  const [acting, setActing] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Daemon | null>(null);

  const fetchDaemons = () => {
    setLoading(true);
    api
      .get<ListResponse<Daemon>>(`/api/v1/webroots/${webrootId}/daemons`)
      .then((res) => {
        setDaemons(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchDaemons();
  }, [webrootId]);

  const handleCreate = async (data: {
    command: string;
    proxy_port?: number;
    num_procs?: number;
  }) => {
    setSaving(true);
    try {
      await api.post(`/api/v1/webroots/${webrootId}/daemons`, data);
      setShowForm(false);
      fetchDaemons();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleUpdate = async (data: {
    command: string;
    proxy_port?: number;
    num_procs?: number;
  }) => {
    if (!editingDaemon) return;
    setSaving(true);
    try {
      await api.put(`/api/v1/daemons/${editingDaemon.id}`, data);
      setEditingDaemon(null);
      fetchDaemons();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleToggle = async (daemon: Daemon) => {
    setActing(daemon.id);
    try {
      const action = daemon.enabled ? "disable" : "enable";
      await api.post(`/api/v1/daemons/${daemon.id}/${action}`, {});
      fetchDaemons();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActing(null);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setActing(deleteTarget.id);
    setDeleteTarget(null);
    try {
      await api.delete(`/api/v1/daemons/${deleteTarget.id}`);
      fetchDaemons();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActing(null);
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

      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">{t("webrootDetail.daemons.title")}</h2>
        {!showForm && !editingDaemon && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("webrootDetail.daemons.addDaemon")}
          </button>
        )}
      </div>

      <PageIntro text={t("webrootDetail.daemons.description")} />

      {showForm && (
        <DaemonForm
          onSave={handleCreate}
          onCancel={() => setShowForm(false)}
          saving={saving}
        />
      )}

      {editingDaemon && (
        <DaemonForm
          onSave={handleUpdate}
          onCancel={() => setEditingDaemon(null)}
          saving={saving}
          initial={editingDaemon}
        />
      )}

      {daemons.length === 0 && !showForm ? (
        <EmptyState icon={Play} message={t("webrootDetail.daemons.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.name")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("webrootDetail.daemons.command")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.status")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.enabled")}
                </th>
                <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.actions")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {daemons.map((d) => (
                <tr key={d.id}>
                  <td className="whitespace-nowrap px-4 py-3 text-sm font-medium text-gray-900">
                    {d.id}
                  </td>
                  <td className="max-w-xs truncate px-4 py-3 font-mono text-sm text-gray-500">
                    {d.command}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    <StatusBadge status={d.status} />
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    <button
                      onClick={() => handleToggle(d)}
                      disabled={acting === d.id}
                      className={`inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${
                        d.enabled
                          ? "bg-green-50 text-green-700 ring-green-600/20 hover:bg-green-100"
                          : "bg-gray-50 text-gray-600 ring-gray-500/20 hover:bg-gray-100"
                      } disabled:opacity-50`}
                    >
                      {d.enabled ? t("common.on") : t("common.off")}
                    </button>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                    <div className="inline-flex items-center gap-3">
                      <button
                        onClick={() => setEditingDaemon(d)}
                        className="text-gray-400 hover:text-brand-600"
                        title={t("common.edit")}
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setDeleteTarget(d)}
                        disabled={acting === d.id}
                        className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                        title={t("common.delete")}
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t("webrootDetail.daemons.deleteTitle")}
        message={t("webrootDetail.daemons.deleteMessage", { name: deleteTarget?.id })}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

function DaemonForm({
  onSave,
  onCancel,
  saving,
  initial,
}: {
  onSave: (data: {
    command: string;
    proxy_port?: number;
    num_procs?: number;
  }) => void;
  onCancel: () => void;
  saving: boolean;
  initial?: Daemon;
}) {
  const { t } = useTranslation();
  const [command, setCommand] = useState(initial?.command || "");
  const [proxyPort, setProxyPort] = useState(
    initial?.proxy_port ? String(initial.proxy_port) : "",
  );
  const [numProcs, setNumProcs] = useState(
    initial?.num_procs ? String(initial.num_procs) : "1",
  );
  const isEdit = !!initial;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!command) return;
    onSave({
      command,
      proxy_port: proxyPort ? parseInt(proxyPort) : undefined,
      num_procs: parseInt(numProcs) || 1,
    });
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      <div className="mb-3 grid gap-3 sm:grid-cols-3">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("webrootDetail.daemons.command")}
          </label>
          <input
            type="text"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            placeholder="node server.js"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 font-mono text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("webrootDetail.daemons.proxyPort")}
          </label>
          <input
            type="number"
            value={proxyPort}
            onChange={(e) => setProxyPort(e.target.value)}
            placeholder="Optional"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("webrootDetail.daemons.processes")}
          </label>
          <input
            type="number"
            value={numProcs}
            onChange={(e) => setNumProcs(e.target.value)}
            min="1"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
      </div>
      <div className="flex justify-end gap-2">
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
          disabled={saving || !command}
          className="inline-flex items-center gap-1 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
        >
          <Check className="h-4 w-4" />
          {saving ? t("common.saving") : isEdit ? t("common.update") : t("common.create")}
        </button>
      </div>
    </form>
  );
}

// ─── Cron Jobs Tab ──────────────────────────────────────────────────────────

function CronJobsTab({ webrootId }: { webrootId: string }) {
  const { t } = useTranslation();
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [editingJob, setEditingJob] = useState<CronJob | null>(null);
  const [saving, setSaving] = useState(false);
  const [acting, setActing] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<CronJob | null>(null);

  const fetchJobs = () => {
    setLoading(true);
    api
      .get<ListResponse<CronJob>>(`/api/v1/webroots/${webrootId}/cron-jobs`)
      .then((res) => {
        setJobs(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchJobs();
  }, [webrootId]);

  const handleCreate = async (data: {
    schedule: string;
    command: string;
  }) => {
    setSaving(true);
    try {
      await api.post(`/api/v1/webroots/${webrootId}/cron-jobs`, data);
      setShowForm(false);
      fetchJobs();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleUpdate = async (data: {
    schedule: string;
    command: string;
  }) => {
    if (!editingJob) return;
    setSaving(true);
    try {
      await api.put(`/api/v1/cron-jobs/${editingJob.id}`, data);
      setEditingJob(null);
      fetchJobs();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleToggle = async (job: CronJob) => {
    setActing(job.id);
    try {
      const action = job.enabled ? "disable" : "enable";
      await api.post(`/api/v1/cron-jobs/${job.id}/${action}`, {});
      fetchJobs();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActing(null);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setActing(deleteTarget.id);
    setDeleteTarget(null);
    try {
      await api.delete(`/api/v1/cron-jobs/${deleteTarget.id}`);
      fetchJobs();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActing(null);
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

      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">{t("webrootDetail.cronJobs.title")}</h2>
        {!showForm && !editingJob && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("webrootDetail.cronJobs.addCronJob")}
          </button>
        )}
      </div>

      <PageIntro text={t("webrootDetail.cronJobs.description")} />

      {showForm && (
        <CronJobForm
          onSave={handleCreate}
          onCancel={() => setShowForm(false)}
          saving={saving}
        />
      )}

      {editingJob && (
        <CronJobForm
          onSave={handleUpdate}
          onCancel={() => setEditingJob(null)}
          saving={saving}
          initial={editingJob}
        />
      )}

      {jobs.length === 0 && !showForm ? (
        <EmptyState icon={Clock} message={t("webrootDetail.cronJobs.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.name")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("webrootDetail.cronJobs.schedule")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("webrootDetail.cronJobs.command")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.enabled")}
                </th>
                <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.actions")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {jobs.map((j) => (
                <tr key={j.id}>
                  <td className="whitespace-nowrap px-4 py-3 text-sm font-medium text-gray-900">
                    {j.id}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-sm text-gray-500">
                    {j.schedule}
                  </td>
                  <td className="max-w-xs truncate px-4 py-3 font-mono text-sm text-gray-500">
                    {j.command}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    <button
                      onClick={() => handleToggle(j)}
                      disabled={acting === j.id}
                      className={`inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${
                        j.enabled
                          ? "bg-green-50 text-green-700 ring-green-600/20 hover:bg-green-100"
                          : "bg-gray-50 text-gray-600 ring-gray-500/20 hover:bg-gray-100"
                      } disabled:opacity-50`}
                    >
                      {j.enabled ? t("common.on") : t("common.off")}
                    </button>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                    <div className="inline-flex items-center gap-3">
                      <button
                        onClick={() => setEditingJob(j)}
                        className="text-gray-400 hover:text-brand-600"
                        title={t("common.edit")}
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setDeleteTarget(j)}
                        disabled={acting === j.id}
                        className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                        title={t("common.delete")}
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t("webrootDetail.cronJobs.deleteTitle")}
        message={t("webrootDetail.cronJobs.deleteMessage", { name: deleteTarget?.id })}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

function CronJobForm({
  onSave,
  onCancel,
  saving,
  initial,
}: {
  onSave: (data: { schedule: string; command: string }) => void;
  onCancel: () => void;
  saving: boolean;
  initial?: CronJob;
}) {
  const { t } = useTranslation();
  const [schedule, setSchedule] = useState(initial?.schedule || "");
  const [command, setCommand] = useState(initial?.command || "");
  const isEdit = !!initial;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!schedule || !command) return;
    onSave({ schedule, command });
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      <div className="mb-3 grid gap-3 sm:grid-cols-2">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("webrootDetail.cronJobs.schedule")}
          </label>
          <FieldHelp text={t("webrootDetail.cronJobs.scheduleHelp")} />
          <input
            type="text"
            value={schedule}
            onChange={(e) => setSchedule(e.target.value)}
            placeholder="*/5 * * * *"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 font-mono text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("webrootDetail.cronJobs.command")}
          </label>
          <input
            type="text"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            placeholder="php artisan schedule:run"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 font-mono text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
      </div>
      <div className="flex justify-end gap-2">
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
          disabled={saving || !schedule || !command}
          className="inline-flex items-center gap-1 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
        >
          <Check className="h-4 w-4" />
          {saving ? t("common.saving") : isEdit ? t("common.update") : t("common.create")}
        </button>
      </div>
    </form>
  );
}

// ─── Hostnames Tab ───────────────────────────────────────────────────────────

function FQDNsTab({ webrootId }: { webrootId: string }) {
  const { t } = useTranslation();
  const [fqdns, setFqdns] = useState<FQDN[]>([]);
  const [available, setAvailable] = useState<FQDN[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showPicker, setShowPicker] = useState(false);
  const [loadingAvailable, setLoadingAvailable] = useState(false);
  const [acting, setActing] = useState<string | null>(null);
  const [detachTarget, setDetachTarget] = useState<FQDN | null>(null);

  const fetchFqdns = () => {
    setLoading(true);
    api
      .get<ListResponse<FQDN>>(`/api/v1/webroots/${webrootId}/fqdns`)
      .then((res) => {
        setFqdns(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchFqdns();
  }, [webrootId]);

  const openPicker = () => {
    setShowPicker(true);
    setLoadingAvailable(true);
    api
      .get<ListResponse<FQDN>>(
        `/api/v1/webroots/${webrootId}/available-fqdns`,
      )
      .then((res) => setAvailable(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoadingAvailable(false));
  };

  const handleAttach = async (fqdnId: string) => {
    setActing(fqdnId);
    try {
      await api.post(
        `/api/v1/webroots/${webrootId}/fqdns/${fqdnId}/attach`,
        {},
      );
      setShowPicker(false);
      fetchFqdns();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActing(null);
    }
  };

  const confirmDetach = async () => {
    if (!detachTarget) return;
    setActing(detachTarget.id);
    setDetachTarget(null);
    try {
      await api.post(
        `/api/v1/webroots/${webrootId}/fqdns/${detachTarget.id}/detach`,
        {},
      );
      fetchFqdns();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setActing(null);
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

      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">{t("webrootDetail.fqdns.title")}</h2>
        {!showPicker && (
          <button
            onClick={openPicker}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("webrootDetail.fqdns.addHostname")}
          </button>
        )}
      </div>

      <PageIntro text={t("webrootDetail.fqdns.description")} />

      {showPicker && (
        <div className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-medium text-gray-900">
              {t("webrootDetail.fqdns.availableHostnames")}
            </h3>
            <button
              onClick={() => setShowPicker(false)}
              className="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-100"
            >
              <X className="h-4 w-4" />
              {t("common.cancel")}
            </button>
          </div>
          {loadingAvailable ? (
            <div className="flex justify-center py-6">
              <div className="h-5 w-5 animate-spin rounded-full border-2 border-brand-600 border-t-transparent" />
            </div>
          ) : available.length === 0 ? (
            <p className="py-4 text-center text-sm text-gray-500">
              {t("webrootDetail.fqdns.noAvailable")}
            </p>
          ) : (
            <div className="space-y-2">
              {available.map((f) => (
                <div
                  key={f.id}
                  className="flex items-center justify-between rounded-lg border border-gray-200 px-4 py-2"
                >
                  <span className="text-sm font-medium text-gray-900">
                    {f.fqdn}
                  </span>
                  <button
                    onClick={() => handleAttach(f.id)}
                    disabled={acting === f.id}
                    className="inline-flex items-center gap-1 rounded-md bg-brand-50 px-3 py-1 text-xs font-medium text-brand-700 ring-1 ring-inset ring-brand-600/20 hover:bg-brand-100 disabled:opacity-50"
                  >
                    {acting === f.id ? "..." : t("common.attach")}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {fqdns.length === 0 && !showPicker ? (
        <EmptyState icon={Globe} message={t("webrootDetail.fqdns.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("webrootDetail.fqdns.hostname")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("webrootDetail.fqdns.ssl")}
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
              {fqdns.map((f) => (
                <tr key={f.id}>
                  <td className="whitespace-nowrap px-4 py-3 text-sm font-medium text-gray-900">
                    {f.fqdn}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    {f.ssl_enabled ? (
                      <span className="inline-flex items-center rounded-md bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700 ring-1 ring-inset ring-green-600/20">
                        {t("common.enabled")}
                      </span>
                    ) : (
                      <span className="inline-flex items-center rounded-md bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-inset ring-gray-500/20">
                        {t("common.disabled")}
                      </span>
                    )}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    <StatusBadge status={f.status} />
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                    <button
                      onClick={() => setDetachTarget(f)}
                      disabled={acting === f.id}
                      className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                      title={t("common.disconnect")}
                    >
                      <Unlink className="inline h-4 w-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={detachTarget !== null}
        title={t("webrootDetail.fqdns.detachTitle")}
        message={t("webrootDetail.fqdns.detachMessage", { name: detachTarget?.fqdn })}
        confirmLabel={t("common.disconnect")}
        variant="danger"
        onConfirm={confirmDetach}
        onCancel={() => setDetachTarget(null)}
      />
    </div>
  );
}

// ─── Env Vars Tab ───────────────────────────────────────────────────────────

function EnvVarsTab({ webrootId }: { webrootId: string }) {
  const { t } = useTranslation();
  const [vars, setVars] = useState<EnvVar[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [editingName, setEditingName] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);

  const fetchVars = () => {
    setLoading(true);
    api
      .get<ListResponse<EnvVar>>(`/api/v1/webroots/${webrootId}/env-vars`)
      .then((res) => {
        setVars(res.items);
        setError("");
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchVars();
  }, [webrootId]);

  const handleSave = async (
    name: string,
    value: string,
    secret: boolean,
    isEdit: boolean,
  ) => {
    setSaving(true);
    try {
      const updated = isEdit
        ? vars.map((v) =>
            v.name === name ? { ...v, value, is_secret: secret } : v,
          )
        : [...vars, { name, value, is_secret: secret }];

      await api.put(`/api/v1/webroots/${webrootId}/env-vars`, {
        vars: updated.map((v) => ({
          name: v.name,
          value: v.value,
          secret: v.is_secret,
        })),
      });

      setShowForm(false);
      setEditingName(null);
      fetchVars();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (name: string) => {
    setDeleting(name);
    try {
      await api.delete(
        `/api/v1/webroots/${webrootId}/env-vars/${encodeURIComponent(name)}`,
      );
      fetchVars();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setDeleting(null);
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

      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">
          {t("webrootDetail.envVars.title")}
        </h2>
        {!showForm && editingName === null && (
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
          >
            <Plus className="h-4 w-4" />
            {t("webrootDetail.envVars.addVariable")}
          </button>
        )}
      </div>

      {showForm && (
        <EnvVarForm
          onSave={(name, value, secret) => handleSave(name, value, secret, false)}
          onCancel={() => setShowForm(false)}
          saving={saving}
          existingNames={vars.map((v) => v.name)}
        />
      )}

      {editingName !== null && (
        <EnvVarForm
          initialName={editingName}
          initialSecret={vars.find((v) => v.name === editingName)?.is_secret}
          onSave={(name, value, secret) => handleSave(name, value, secret, true)}
          onCancel={() => setEditingName(null)}
          saving={saving}
          isEdit
        />
      )}

      {vars.length === 0 && !showForm ? (
        <EmptyState icon={Settings} message={t("webrootDetail.envVars.empty")} />
      ) : (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.name")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("webrootDetail.envVars.value")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.type")}
                </th>
                <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">
                  {t("common.actions")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {vars.map((v) => (
                <tr key={v.name}>
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-sm text-gray-900">
                    {v.name}
                  </td>
                  <td className="max-w-xs truncate px-4 py-3 font-mono text-sm text-gray-500">
                    {v.is_secret ? (
                      <span className="inline-flex items-center gap-1 text-gray-400">
                        <Lock className="h-3 w-3" />
                        ***
                      </span>
                    ) : (
                      <span title={v.value}>{v.value}</span>
                    )}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-sm">
                    {v.is_secret ? (
                      <span className="inline-flex items-center rounded-md bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 ring-1 ring-inset ring-amber-600/20">
                        {t("common.secret")}
                      </span>
                    ) : (
                      <span className="inline-flex items-center rounded-md bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600 ring-1 ring-inset ring-gray-500/20">
                        {t("common.plain")}
                      </span>
                    )}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-right text-sm">
                    <div className="inline-flex items-center gap-3">
                      <button
                        onClick={() => setEditingName(v.name)}
                        className="text-gray-400 hover:text-brand-600"
                        title={t("common.edit")}
                      >
                        {t("common.edit")}
                      </button>
                      <button
                        onClick={() => setDeleteTarget(v.name)}
                        disabled={deleting === v.name}
                        className="text-gray-400 hover:text-red-600 disabled:opacity-50"
                        title={t("common.delete")}
                      >
                        {deleting === v.name ? (
                          <span className="text-xs">...</span>
                        ) : (
                          <Trash2 className="inline h-4 w-4" />
                        )}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete environment variable"
        message={`Are you sure you want to delete "${deleteTarget}"? This action cannot be undone.`}
        confirmLabel={t("common.delete")}
        variant="danger"
        onConfirm={() => {
          if (deleteTarget) {
            handleDelete(deleteTarget);
            setDeleteTarget(null);
          }
        }}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

function EnvVarForm({
  initialName,
  initialSecret,
  onSave,
  onCancel,
  saving,
  isEdit,
  existingNames,
}: {
  initialName?: string;
  initialSecret?: boolean;
  onSave: (name: string, value: string, secret: boolean) => void;
  onCancel: () => void;
  saving: boolean;
  isEdit?: boolean;
  existingNames?: string[];
}) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialName || "");
  const [value, setValue] = useState("");
  const [secret, setSecret] = useState(initialSecret || false);
  const [nameError, setNameError] = useState("");

  const validateName = (n: string) => {
    if (!n) return t("webrootDetail.envVars.nameRequired");
    if (!ENV_NAME_RE.test(n)) return t("webrootDetail.envVars.nameInvalid");
    if (!isEdit && existingNames?.includes(n)) return t("webrootDetail.envVars.nameExists");
    return "";
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const err = validateName(name);
    if (err) {
      setNameError(err);
      return;
    }
    if (!value) return;
    onSave(name, value, secret);
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-4 rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
    >
      <div className="mb-3 grid gap-3 sm:grid-cols-2">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("common.name")}
          </label>
          <input
            type="text"
            value={name}
            onChange={(e) => {
              setName(e.target.value.toUpperCase());
              setNameError("");
            }}
            disabled={isEdit}
            placeholder="MY_VARIABLE"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 disabled:bg-gray-50 disabled:text-gray-500"
          />
          {nameError && (
            <p className="mt-1 text-xs text-red-600">{nameError}</p>
          )}
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700">
            {t("webrootDetail.envVars.value")}
          </label>
          <input
            type={secret ? "password" : "text"}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder={isEdit ? t("webrootDetail.envVars.newValuePlaceholder") : "Value"}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>
      </div>
      <div className="flex items-center justify-between">
        <label className="flex items-center gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            checked={secret}
            onChange={(e) => setSecret(e.target.checked)}
            className="rounded border-gray-300 text-brand-600 focus:ring-brand-500"
          />
          {t("webrootDetail.envVars.secretLabel")}
        </label>
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
            disabled={saving || !value}
            className="inline-flex items-center gap-1 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
          >
            <Check className="h-4 w-4" />
            {saving ? t("common.saving") : isEdit ? t("common.update") : t("common.add")}
          </button>
        </div>
      </div>
    </form>
  );
}

// ─── Settings Tab ───────────────────────────────────────────────────────────

function SettingsTab({
  webrootId,
  webroot,
  onUpdate,
}: {
  webrootId: string;
  webroot: Webroot;
  onUpdate: (w: Webroot) => void;
}) {
  const { t } = useTranslation();
  const [runtimes, setRuntimes] = useState<RuntimeGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [runtime, setRuntime] = useState(webroot.runtime);
  const [runtimeVersion, setRuntimeVersion] = useState(webroot.runtime_version);
  const [publicFolder, setPublicFolder] = useState(webroot.public_folder);

  useEffect(() => {
    api
      .get<ListResponse<RuntimeGroup>>(
        `/api/v1/webroots/${webrootId}/runtimes`,
      )
      .then((res) => setRuntimes(res.items))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [webrootId]);

  const selectedGroup = runtimes.find((r) => r.runtime === runtime);
  const versions = selectedGroup?.versions ?? [];

  // When runtime changes, auto-select the first available version
  const handleRuntimeChange = (newRuntime: string) => {
    setRuntime(newRuntime);
    const group = runtimes.find((r) => r.runtime === newRuntime);
    if (group && group.versions.length > 0) {
      setRuntimeVersion(group.versions[0]);
    } else {
      setRuntimeVersion("");
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError("");
    setSuccess("");
    try {
      const updated = await api.put<Webroot>(
        `/api/v1/webroots/${webrootId}`,
        { runtime, runtime_version: runtimeVersion, public_folder: publicFolder },
      );
      onUpdate(updated);
      setSuccess(t("webrootDetail.settings.saved"));
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <TabSkeleton />
    );
  }

  return (
    <div>
      <h2 className="mb-2 text-lg font-semibold text-gray-900">
        {t("webrootDetail.settings.title")}
      </h2>
      <PageIntro text={t("webrootDetail.settings.description")} />

      {error && <ErrorAlert message={error} />}
      {success && (
        <div className="mb-4 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">
          {success}
        </div>
      )}

      <form
        onSubmit={handleSave}
        className="rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
      >
        <div className="mb-4 grid gap-4 sm:grid-cols-3">
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-700">
              {t("webrootDetail.settings.runtime")}
            </label>
            <select
              value={runtime}
              onChange={(e) => handleRuntimeChange(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
            >
              {runtimes.map((r) => (
                <option key={r.runtime} value={r.runtime}>
                  {r.runtime}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-700">
              {t("webrootDetail.settings.version")}
            </label>
            <select
              value={runtimeVersion}
              onChange={(e) => setRuntimeVersion(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
            >
              {versions.map((v) => (
                <option key={v} value={v}>
                  {v}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-700">
              {t("webrootDetail.settings.publicFolder")}
            </label>
            <input
              type="text"
              value={publicFolder}
              onChange={(e) => setPublicFolder(e.target.value)}
              placeholder="/public"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 font-mono text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
            />
          </div>
        </div>
        <div className="flex justify-end">
          <button
            type="submit"
            disabled={saving}
            className="inline-flex items-center gap-1 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
          >
            <Check className="h-4 w-4" />
            {saving ? t("common.saving") : t("webrootDetail.settings.saveSettings")}
          </button>
        </div>
      </form>
    </div>
  );
}

// ─── Vault Tab ──────────────────────────────────────────────────────────────

function VaultTab({ webrootId }: { webrootId: string }) {
  const { t } = useTranslation();
  const [plaintext, setPlaintext] = useState("");
  const [token, setToken] = useState("");
  const [encrypting, setEncrypting] = useState(false);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);

  const handleEncrypt = async () => {
    if (!plaintext.trim()) return;
    setEncrypting(true);
    setError("");
    setToken("");
    try {
      const res = await api.post<VaultEncryptResponse>(
        `/api/v1/webroots/${webrootId}/vault/encrypt`,
        { plaintext },
      );
      setToken(res.token);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setEncrypting(false);
    }
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(token);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">
        {t("webrootDetail.vault.title")}
      </h2>

      <div className="rounded-xl bg-blue-50 p-4 text-sm text-blue-700 ring-1 ring-inset ring-blue-200">
        Encrypt a secret value to get a vault token. Paste the token into your{" "}
        <code className="rounded bg-blue-100 px-1">.env</code> file in git.
        Values prefixed with{" "}
        <code className="rounded bg-blue-100 px-1">vault:v1:</code> are
        encrypted and can be safely committed.
      </div>

      {error && <ErrorAlert message={error} />}

      <div className="mt-4">
        <label className="mb-1 block text-sm font-medium text-gray-700">
          {t("webrootDetail.vault.secretValue")}
        </label>
        <textarea
          value={plaintext}
          onChange={(e) => setPlaintext(e.target.value)}
          rows={4}
          placeholder={t("webrootDetail.vault.secretPlaceholder")}
          className="w-full rounded-lg border border-gray-300 px-3 py-2 font-mono text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
        />
      </div>

      <button
        onClick={handleEncrypt}
        disabled={encrypting || !plaintext.trim()}
        className="mt-3 inline-flex items-center gap-2 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
      >
        <Lock className="h-4 w-4" />
        {encrypting ? t("webrootDetail.vault.encrypting") : t("webrootDetail.vault.encrypt")}
      </button>

      {token && (
        <div className="mt-6">
          <label className="mb-1 block text-sm font-medium text-gray-700">
            {t("webrootDetail.vault.vaultToken")}
          </label>
          <div className="flex gap-2">
            <input
              type="text"
              value={token}
              readOnly
              className="flex-1 rounded-lg border border-gray-300 bg-gray-50 px-3 py-2 font-mono text-sm text-gray-700"
            />
            <button
              onClick={handleCopy}
              className="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
            >
              {copied ? (
                <>
                  <Check className="h-4 w-4 text-green-600" />
                  {t("common.copied")}
                </>
              ) : (
                <>
                  <ClipboardCopy className="h-4 w-4" />
                  {t("common.copy")}
                </>
              )}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
