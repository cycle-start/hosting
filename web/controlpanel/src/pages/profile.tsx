import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Check } from "lucide-react";
import { useAuth } from "@/auth/context";
import { api } from "@/api/client";
import type { User } from "@/api/types";
import { PageIntro } from "@/components/page-intro";

const LANGUAGES = [
  { code: "en", label: "English" },
  { code: "de", label: "Deutsch" },
  { code: "nb", label: "Norsk bokmål" },
];

export function ProfilePage() {
  const { t } = useTranslation();
  const { user, updateUser } = useAuth();
  const [displayName, setDisplayName] = useState(user?.display_name || "");
  const [locale, setLocale] = useState(user?.locale || "en");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

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

  if (!user) return null;

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
    </div>
  );
}
