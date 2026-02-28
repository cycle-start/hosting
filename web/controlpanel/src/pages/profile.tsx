import { useEffect, useState } from "react";
import { useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Check, Link, Unlink } from "lucide-react";
import { useAuth } from "@/auth/context";
import { api } from "@/api/client";
import type { User, OIDCProvider, OIDCConnection, ListResponse } from "@/api/types";
import { PageIntro } from "@/components/page-intro";

const LANGUAGES = [
  { code: "en", label: "English" },
  { code: "de", label: "Deutsch" },
  { code: "nb", label: "Norsk bokmål" },
];

export function ProfilePage() {
  const { t } = useTranslation();
  const { user, updateUser } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [displayName, setDisplayName] = useState(user?.display_name || "");
  const [locale, setLocale] = useState(user?.locale || "en");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  // OIDC state
  const [oidcProviders, setOidcProviders] = useState<OIDCProvider[]>([]);
  const [oidcConnections, setOidcConnections] = useState<OIDCConnection[]>([]);
  const [oidcSuccess, setOidcSuccess] = useState("");
  const [oidcError, setOidcError] = useState("");

  // Fetch OIDC providers and connections
  useEffect(() => {
    api
      .get<ListResponse<OIDCProvider>>("/auth/oidc/providers")
      .then((res) => setOidcProviders(res.items))
      .catch(() => {});
    api
      .get<ListResponse<OIDCConnection>>("/api/v1/me/oidc-connections")
      .then((res) => setOidcConnections(res.items))
      .catch(() => {});
  }, []);

  // Handle OIDC callback params
  useEffect(() => {
    if (searchParams.get("oidc") === "connected") {
      setOidcSuccess(t("profile.oidcConnected"));
      setSearchParams({}, { replace: true });
      // Refresh connections
      api
        .get<ListResponse<OIDCConnection>>("/api/v1/me/oidc-connections")
        .then((res) => setOidcConnections(res.items))
        .catch(() => {});
    }
    if (searchParams.get("oidc_error")) {
      setOidcError(t("profile.oidcConnectionFailed"));
      setSearchParams({}, { replace: true });
    }
  }, [searchParams, setSearchParams, t]);

  const handleSave = async () => {
    setSaving(true);
    setError("");
    setSuccess(false);
    try {
      const updated = await api.patch<User>("/api/v1/me", {
        locale,
        display_name: displayName || null,
      });
      updateUser(updated);
      setSuccess(true);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleConnect = async (providerId: string) => {
    try {
      const res = await api.post<{ redirect_url: string }>(
        `/api/v1/me/oidc-connections/authorize?provider=${providerId}`,
        {},
      );
      window.location.href = res.redirect_url;
    } catch (err: any) {
      setOidcError(err.message);
    }
  };

  const handleDisconnect = async (providerId: string) => {
    try {
      await api.delete(`/api/v1/me/oidc-connections/${providerId}`);
      setOidcConnections((prev) => prev.filter((c) => c.provider !== providerId));
    } catch (err: any) {
      setOidcError(err.message);
    }
  };

  if (!user) return null;

  const getConnection = (providerId: string) =>
    oidcConnections.find((c) => c.provider === providerId);

  return (
    <div className="p-8">
      <h1 className="mb-2 text-2xl font-bold text-gray-900">
        {t("profile.title")}
      </h1>
      <PageIntro text={t("profile.description")} />

      {error && (
        <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      {success && (
        <div className="mb-4 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">
          {t("profile.saved")}
        </div>
      )}

      <div className="max-w-lg rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200">
        <div className="mb-4">
          <label className="mb-1 block text-sm font-medium text-gray-700">
            {t("profile.displayName")}
          </label>
          <input
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder={t("profile.displayNamePlaceholder")}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          />
        </div>

        <div className="mb-4">
          <label className="mb-1 block text-sm font-medium text-gray-700">
            {t("profile.emailAddress")}
          </label>
          <input
            type="email"
            value={user.email}
            disabled
            className="w-full rounded-lg border border-gray-300 bg-gray-50 px-3 py-2 text-sm text-gray-500"
          />
        </div>

        <div className="mb-6">
          <label className="mb-1 block text-sm font-medium text-gray-700">
            {t("profile.language")}
          </label>
          <select
            value={locale}
            onChange={(e) => setLocale(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
          >
            {LANGUAGES.map((lang) => (
              <option key={lang.code} value={lang.code}>
                {lang.label}
              </option>
            ))}
          </select>
        </div>

        <button
          onClick={handleSave}
          disabled={saving}
          className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
        >
          <Check className="h-4 w-4" />
          {saving ? t("common.saving") : t("profile.saveChanges")}
        </button>
      </div>

      {oidcProviders.length > 0 && (
        <div className="mt-8">
          <h2 className="mb-2 text-lg font-semibold text-gray-900">
            {t("profile.connectedAccounts")}
          </h2>
          <p className="mb-4 text-sm text-gray-500">
            {t("profile.connectedAccountsDescription")}
          </p>

          {oidcSuccess && (
            <div className="mb-4 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">
              {oidcSuccess}
            </div>
          )}

          {oidcError && (
            <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">
              {oidcError}
            </div>
          )}

          <div className="max-w-lg space-y-3">
            {oidcProviders.map((provider) => {
              const conn = getConnection(provider.id);
              return (
                <div
                  key={provider.id}
                  className="flex items-center justify-between rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
                >
                  <div>
                    <p className="text-sm font-medium text-gray-900">
                      {provider.name}
                    </p>
                    {conn && (
                      <p className="text-xs text-gray-500">{conn.email}</p>
                    )}
                  </div>
                  {conn ? (
                    <button
                      onClick={() => handleDisconnect(provider.id)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50"
                    >
                      <Unlink className="h-3.5 w-3.5" />
                      {t("common.disconnect")}
                    </button>
                  ) : (
                    <button
                      onClick={() => handleConnect(provider.id)}
                      className="inline-flex items-center gap-1.5 rounded-lg bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
                    >
                      <Link className="h-3.5 w-3.5" />
                      {t("profile.connect")}
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
