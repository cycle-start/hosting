import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight, RefreshCw, TriangleAlert } from "lucide-react";

export function ErrorPage({ error }: { error: string }) {
  const { t } = useTranslation();
  const [showDetails, setShowDetails] = useState(false);

  return (
    <div className="p-8">
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <TriangleAlert className="mb-4 h-12 w-12 text-amber-500" />
        <h1 className="mb-2 text-xl font-semibold text-gray-900">
          {t("error.title")}
        </h1>
        <p className="mb-6 max-w-md text-sm text-gray-500">
          {t("error.description")}
        </p>
        <div className="flex items-center gap-3">
          <button
            onClick={() => window.location.reload()}
            className="inline-flex items-center gap-2 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700"
          >
            <RefreshCw className="h-4 w-4" />
            {t("error.tryAgain")}
          </button>
          <a
            href="mailto:support@example.com"
            className="inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium text-gray-600 hover:bg-gray-100"
          >
            {t("error.contactSupport")}
          </a>
        </div>
        <div className="mt-8 w-full max-w-md">
          <button
            onClick={() => setShowDetails((v) => !v)}
            className="inline-flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600"
          >
            {showDetails ? (
              <ChevronDown className="h-3 w-3" />
            ) : (
              <ChevronRight className="h-3 w-3" />
            )}
            {t("error.details")}
          </button>
          {showDetails && (
            <pre className="mt-2 rounded-lg bg-gray-100 px-4 py-3 text-left text-xs text-gray-600">
              {error}
            </pre>
          )}
        </div>
      </div>
    </div>
  );
}
