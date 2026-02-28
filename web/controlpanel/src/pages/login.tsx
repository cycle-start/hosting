import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Navigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { useAuth } from "@/auth/context";
import { api } from "@/api/client";
import type { OIDCProvider, ListResponse } from "@/api/types";
import { LogIn } from "lucide-react";

export function LoginPage() {
  const { t } = useTranslation();
  const { login, loginWithToken, isAuthenticated, isLoading, user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [oidcProviders, setOidcProviders] = useState<OIDCProvider[]>([]);

  // Handle OIDC token from callback redirect
  useEffect(() => {
    const token = searchParams.get("token");
    if (token) {
      loginWithToken(token);
      setSearchParams({}, { replace: true });
    }
  }, [searchParams, loginWithToken, setSearchParams]);

  // Handle OIDC error from callback redirect
  useEffect(() => {
    const oidcError = searchParams.get("oidc_error");
    if (oidcError === "no_account") {
      setError(t("login.oidcNoAccount"));
      setSearchParams({}, { replace: true });
    } else if (oidcError) {
      setError(t("login.oidcError"));
      setSearchParams({}, { replace: true });
    }
  }, [searchParams, setSearchParams, t]);

  // Fetch OIDC providers
  useEffect(() => {
    api
      .get<ListResponse<OIDCProvider>>("/auth/oidc/providers")
      .then((res) => setOidcProviders(res.items))
      .catch(() => {});
  }, []);

  if (isLoading) return null;
  if (isAuthenticated) {
    const dest = user?.last_customer_id
      ? `/customers/${user.last_customer_id}`
      : "/customers";
    return <Navigate to={dest} replace />;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setSubmitting(true);

    try {
      await login(email, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("login.failed"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold text-gray-900">{t("login.title")}</h2>

      {error && (
        <div className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      {oidcProviders.length > 0 && (
        <>
          <div className="space-y-2">
            {oidcProviders.map((provider) => (
              <a
                key={provider.id}
                href={`/auth/oidc/authorize?provider=${provider.id}`}
                className="flex w-full items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 focus:ring-2 focus:ring-brand-500/20 focus:outline-none"
              >
                {t("login.signInWith", { provider: provider.name })}
              </a>
            ))}
          </div>
          <div className="relative">
            <div className="absolute inset-0 flex items-center">
              <div className="w-full border-t border-gray-300" />
            </div>
            <div className="relative flex justify-center text-sm">
              <span className="bg-white px-2 text-gray-500">{t("login.or")}</span>
            </div>
          </div>
        </>
      )}

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="email" className="mb-1 block text-sm font-medium text-gray-700">
            {t("login.email")}
          </label>
          <input
            id="email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-brand-500 focus:ring-2 focus:ring-brand-500/20 focus:outline-none"
            placeholder="admin@acme-hosting.test"
          />
        </div>

        <div>
          <label htmlFor="password" className="mb-1 block text-sm font-medium text-gray-700">
            {t("login.password")}
          </label>
          <input
            id="password"
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-brand-500 focus:ring-2 focus:ring-brand-500/20 focus:outline-none"
            placeholder="••••••••"
          />
        </div>

        <button
          type="submit"
          disabled={submitting}
          className="flex w-full items-center justify-center gap-2 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-brand-700 focus:ring-2 focus:ring-brand-500/20 focus:outline-none disabled:opacity-50"
        >
          {submitting ? (
            <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
          ) : (
            <LogIn className="h-4 w-4" />
          )}
          {t("login.submit")}
        </button>
      </form>
    </div>
  );
}
