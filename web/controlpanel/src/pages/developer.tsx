import { useTranslation } from "react-i18next";
import { usePartner } from "@/partner/context";
import { PageIntro } from "@/components/page-intro";

const MCP_GROUPS = [
  { name: "account", description: "Authentication, profile, and customer management" },
  { name: "webroots", description: "Manage webroots, environment variables, daemons, and cron jobs" },
  { name: "databases", description: "Manage MySQL databases and database users" },
  { name: "dns", description: "Manage DNS zones and zone records" },
  { name: "email", description: "Manage email accounts, aliases, forwards, and autoreplies" },
  { name: "storage", description: "Manage S3 buckets, access keys, and Valkey instances" },
  { name: "access", description: "Manage SSH keys and backups" },
];

export function DeveloperPage() {
  const { t } = useTranslation();
  const { partner } = usePartner();
  const baseUrl = partner?.hostname
    ? `https://${partner.hostname}`
    : window.location.origin;

  const mcpConfig = JSON.stringify(
    {
      mcpServers: Object.fromEntries(
        MCP_GROUPS.map((g) => [
          `controlpanel-${g.name}`,
          {
            type: "streamable-http",
            url: `${baseUrl}/mcp/${g.name}`,
          },
        ]),
      ),
    },
    null,
    2,
  );

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold">{t("developer.title")}</h1>
      <PageIntro text={t("developer.description")} />

      {/* API Documentation */}
      <section className="mt-8">
        <h2 className="text-lg font-semibold">{t("developer.apiDocs.title")}</h2>
        <p className="mt-1 text-sm text-gray-600">
          {t("developer.apiDocs.description")}
        </p>
        <a
          href="/docs"
          target="_blank"
          rel="noopener noreferrer"
          className="mt-3 inline-block rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700"
        >
          {t("developer.apiDocs.viewSpec")}
        </a>
      </section>

      {/* MCP Integration */}
      <section className="mt-10">
        <h2 className="text-lg font-semibold">{t("developer.mcp.title")}</h2>
        <p className="mt-1 text-sm text-gray-600">
          {t("developer.mcp.description")}
        </p>

        {/* Setup Steps */}
        <div className="mt-6 space-y-4">
          <h3 className="font-medium">{t("developer.mcp.setup.title")}</h3>

          <div className="rounded-lg border bg-gray-50 p-4">
            <p className="text-sm font-medium">{t("developer.mcp.setup.step1")}</p>
            <p className="mt-1 text-xs text-gray-500">
              {t("developer.mcp.setup.step1Detail")}
            </p>
          </div>

          <div className="rounded-lg border bg-gray-50 p-4">
            <p className="text-sm font-medium">{t("developer.mcp.setup.step2")}</p>
            <p className="mt-1 text-xs text-gray-500">
              {t("developer.mcp.setup.step2Detail")}
            </p>
            <pre className="mt-3 overflow-x-auto rounded-md bg-gray-900 p-4 text-xs text-gray-100">
              {mcpConfig}
            </pre>
          </div>

          <div className="rounded-lg border bg-gray-50 p-4">
            <p className="text-sm font-medium">{t("developer.mcp.setup.step3")}</p>
            <p className="mt-1 text-xs text-gray-500">
              {t("developer.mcp.setup.step3Detail")}
            </p>
          </div>
        </div>

        {/* Tool Groups */}
        <div className="mt-8">
          <h3 className="font-medium">{t("developer.mcp.groups.title")}</h3>
          <div className="mt-3 overflow-hidden rounded-lg border">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    {t("developer.mcp.groups.endpoint")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    {t("developer.mcp.groups.description")}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {MCP_GROUPS.map((group) => (
                  <tr key={group.name}>
                    <td className="whitespace-nowrap px-4 py-3 font-mono text-sm">
                      /mcp/{group.name}
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-600">
                      {group.description}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>
  );
}
