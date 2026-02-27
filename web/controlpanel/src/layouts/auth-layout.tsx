import { Outlet } from "react-router";
import { Shield } from "lucide-react";
import { usePartner } from "@/partner/context";

export function AuthLayout() {
  const { partner } = usePartner();

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-100 p-4">
      <div className="w-full max-w-md">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-brand-600 text-white">
            <Shield className="h-6 w-6" />
          </div>
          <h1 className="text-2xl font-bold text-gray-900">
            {partner?.name ?? "Control Panel"}
          </h1>
        </div>
        <div className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-gray-200">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
