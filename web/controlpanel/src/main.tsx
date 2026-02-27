import "@/i18n";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router";
import { PartnerProvider } from "@/partner/context";
import { AuthProvider } from "@/auth/context";
import { RequireAuth } from "@/auth/guard";
import { AuthLayout } from "@/layouts/auth-layout";
import { AppLayout } from "@/layouts/app-layout";
import { LoginPage } from "@/pages/login";
import { CustomersPage } from "@/pages/customers";
import { DashboardPage } from "@/pages/dashboard";
import { WebrootsPage } from "@/pages/webroots";
import { WebrootDetailPage } from "@/pages/webroot-detail";
import { DatabasesPage } from "@/pages/databases";
import { DatabaseDetailPage } from "@/pages/database-detail";
import { ValkeyPage } from "@/pages/valkey";
import { ValkeyDetailPage } from "@/pages/valkey-detail";
import { WireGuardPage } from "@/pages/wireguard";
import { WireGuardDetailPage } from "@/pages/wireguard-detail";
import { S3BucketsPage } from "@/pages/s3-buckets";
import { S3BucketDetailPage } from "@/pages/s3-bucket-detail";
import { EmailPage } from "@/pages/email";
import { EmailDetailPage } from "@/pages/email-detail";
import { DNSPage } from "@/pages/dns";
import { DNSDetailPage } from "@/pages/dns-detail";
import { SSHKeysPage } from "@/pages/ssh-keys";
import { BackupsPage } from "@/pages/backups";
import { ProfilePage } from "@/pages/profile";
import { DeveloperPage } from "@/pages/developer";
import { DevToolbar } from "@/components/dev-toolbar";
import { getLastCustomerId } from "@/lib/last-customer";
import "./index.css";

function DefaultRedirect() {
  const lastId = getLastCustomerId();
  return <Navigate to={lastId ? `/customers/${lastId}` : "/customers"} replace />;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <PartnerProvider>
      <AuthProvider>
        <BrowserRouter>
        <Routes>
          {/* Public routes */}
          <Route element={<AuthLayout />}>
            <Route path="/login" element={<LoginPage />} />
          </Route>

          {/* Authenticated routes */}
          <Route element={<RequireAuth />}>
            <Route element={<AppLayout />}>
              <Route path="/customers" element={<CustomersPage />} />
              <Route path="/customers/:id" element={<DashboardPage />} />
              <Route
                path="/customers/:id/webroots"
                element={<WebrootsPage />}
              />
              <Route
                path="/customers/:id/webroots/:webrootId"
                element={<WebrootDetailPage />}
              />
              <Route
                path="/customers/:id/databases"
                element={<DatabasesPage />}
              />
              <Route
                path="/customers/:id/databases/:databaseId"
                element={<DatabaseDetailPage />}
              />
              <Route
                path="/customers/:id/valkey"
                element={<ValkeyPage />}
              />
              <Route
                path="/customers/:id/valkey/:instanceId"
                element={<ValkeyDetailPage />}
              />
              <Route
                path="/customers/:id/wireguard"
                element={<WireGuardPage />}
              />
              <Route
                path="/customers/:id/wireguard/:peerId"
                element={<WireGuardDetailPage />}
              />
              <Route
                path="/customers/:id/s3"
                element={<S3BucketsPage />}
              />
              <Route
                path="/customers/:id/s3/:bucketId"
                element={<S3BucketDetailPage />}
              />
              <Route
                path="/customers/:id/email"
                element={<EmailPage />}
              />
              <Route
                path="/customers/:id/email/:accountId"
                element={<EmailDetailPage />}
              />
              <Route
                path="/customers/:id/dns"
                element={<DNSPage />}
              />
              <Route
                path="/customers/:id/dns/:zoneId"
                element={<DNSDetailPage />}
              />
              <Route
                path="/customers/:id/ssh-keys"
                element={<SSHKeysPage />}
              />
              <Route
                path="/customers/:id/backups"
                element={<BackupsPage />}
              />
              <Route path="/profile" element={<ProfilePage />} />
              <Route path="/developer" element={<DeveloperPage />} />
            </Route>
          </Route>

          {/* Default redirect */}
          <Route path="*" element={<DefaultRedirect />} />
        </Routes>
        </BrowserRouter>
      </AuthProvider>
      {import.meta.env.DEV && <DevToolbar />}
    </PartnerProvider>
  </StrictMode>,
);
