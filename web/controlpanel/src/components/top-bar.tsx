import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { LogOut, UserCircle } from "lucide-react";
import { useAuth } from "@/auth/context";
import { useParams } from "react-router";
import { useCustomerModules } from "@/hooks/use-customer-modules";
import { CustomerSwitcher } from "@/components/customer-switcher";

export function TopBar() {
  const { t } = useTranslation();
  const { user, logout } = useAuth();
  const { id } = useParams<{ id: string }>();
  const { customerName } = useCustomerModules();

  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-gray-200 bg-white px-6">
      {/* Left: Customer Switcher */}
      <CustomerSwitcher
        currentCustomerId={id}
        currentCustomerName={customerName}
      />

      {/* Right: Profile + Logout */}
      <div className="flex items-center gap-1">
        {user && (
          <>
            <Link
              to="/profile"
              className="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900"
              title={t("nav.profile")}
            >
              <UserCircle className="h-4 w-4" />
              <span className="hidden sm:inline">
                {user.display_name || user.email}
              </span>
            </Link>
            <button
              onClick={logout}
              className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
              title={t("common.signOut")}
            >
              <LogOut className="h-4 w-4" />
            </button>
          </>
        )}
      </div>
    </header>
  );
}
