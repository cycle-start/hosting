import { useEffect } from "react";
import { Link, NavLink, Outlet, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { LayoutDashboard, Shield, Code } from "lucide-react";
import { useAuth } from "@/auth/context";
import { usePartner } from "@/partner/context";
import { setLastCustomerId } from "@/lib/last-customer";
import {
  CustomerModulesProvider,
  useCustomerModules,
} from "@/hooks/use-customer-modules";
import { TopBar } from "@/components/top-bar";
import { MODULE_META } from "@/lib/modules";

function SidebarLink({
  to,
  icon: Icon,
  children,
  end,
}: {
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  children: React.ReactNode;
  end?: boolean;
}) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        `flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
          isActive
            ? "bg-sidebar-active text-sidebar-text-active"
            : "text-sidebar-text hover:bg-sidebar-hover hover:text-sidebar-text-active"
        }`
      }
    >
      <Icon className="h-4 w-4 shrink-0" />
      {children}
    </NavLink>
  );
}

function ModuleLinks({ customerId }: { customerId: string }) {
  const { t } = useTranslation();
  const { modules } = useCustomerModules();

  const visible = Object.entries(MODULE_META).filter(([key]) =>
    modules.includes(key),
  );

  if (visible.length === 0) return null;

  return (
    <>
      {visible.map(([key, meta]) => (
        <SidebarLink
          key={key}
          to={`/customers/${customerId}/${meta.route}`}
          icon={meta.icon}
        >
          {t(meta.labelKey)}
        </SidebarLink>
      ))}
    </>
  );
}

export function AppLayout() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { partner } = usePartner();
  const { id } = useParams<{ id: string }>();

  const effectiveId = id || user?.last_customer_id;

  // Remember the current customer so we can redirect back on next visit.
  useEffect(() => {
    if (id) setLastCustomerId(id);
  }, [id]);

  return (
    <CustomerModulesProvider customerId={effectiveId ?? undefined}>
      <div className="flex h-screen">
        {/* Sidebar */}
        <aside className="flex w-64 shrink-0 flex-col bg-sidebar">
          {/* Logo */}
          <div className="flex h-14 items-center gap-3 px-4">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-600 text-white">
              <Shield className="h-4 w-4" />
            </div>
            <Link
              to={effectiveId ? `/customers/${effectiveId}` : "/customers"}
              className="text-sm font-semibold text-sidebar-text-active"
            >
              {partner?.name ?? "Control Panel"}
            </Link>
          </div>

          {/* Navigation */}
          <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-2">
            {effectiveId && (
              <>
                <SidebarLink to={`/customers/${effectiveId}`} icon={LayoutDashboard} end>
                  {t("nav.dashboard")}
                </SidebarLink>
                <ModuleLinks customerId={effectiveId} />
              </>
            )}
          </nav>

          {/* Developer */}
          <div className="border-t border-white/10 px-3 py-2">
            <SidebarLink to="/developer" icon={Code}>
              {t("nav.developer")}
            </SidebarLink>
          </div>
        </aside>

        {/* Main content */}
        <div className="flex flex-1 flex-col">
          <TopBar />
          <main className="flex-1 overflow-y-auto">
            <Outlet />
          </main>
        </div>
      </div>
    </CustomerModulesProvider>
  );
}
