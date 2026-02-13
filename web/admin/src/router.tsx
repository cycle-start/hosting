import { createRouter, createRootRoute, createRoute, redirect, Outlet } from '@tanstack/react-router'
import { api } from '@/lib/api'
import { AuthLayout } from './app'
import { LoginPage } from './pages/login'
import { DashboardPage } from './pages/dashboard'
import { BrandsPage } from './pages/brands'
import { BrandDetailPage } from './pages/brand-detail'
import { RegionsPage } from './pages/regions'
import { ClustersPage } from './pages/clusters'
import { TenantsPage } from './pages/tenants'
import { CreateTenantPage } from './pages/create-tenant'
import { TenantDetailPage } from './pages/tenant-detail'
import { WebrootsPage } from './pages/webroots'
import { WebrootDetailPage } from './pages/webroot-detail'
import { FQDNDetailPage } from './pages/fqdn-detail'
import { EmailAccountDetailPage } from './pages/email-account-detail'
import { DatabasesPage } from './pages/databases'
import { DatabaseDetailPage } from './pages/database-detail'
import { ZonesPage } from './pages/zones'
import { ZoneDetailPage } from './pages/zone-detail'
import { ValkeyPage } from './pages/valkey'
import { ValkeyDetailPage } from './pages/valkey-detail'
import { S3BucketsPage } from './pages/s3-buckets'
import { S3BucketDetailPage } from './pages/s3-bucket-detail'
import { PlatformConfigPage } from './pages/platform-config'
import { APIKeysPage } from './pages/api-keys'
import { AuditLogPage } from './pages/audit-log'

const rootRoute = createRootRoute({
  component: Outlet,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
})

const authLayout = createRoute({
  getParentRoute: () => rootRoute,
  id: 'auth',
  beforeLoad: () => {
    if (!api.isAuthenticated()) {
      throw redirect({ to: '/login' })
    }
  },
  component: AuthLayout,
})

const dashboardRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/',
  component: DashboardPage,
})

const brandsRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/brands',
  component: BrandsPage,
})

const brandDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/brands/$id',
  component: BrandDetailPage,
})

const regionsRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/regions',
  component: RegionsPage,
})

const clustersRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/clusters',
  component: ClustersPage,
})

const tenantsRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants',
  component: TenantsPage,
})

const createTenantRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/new',
  component: CreateTenantPage,
})

const tenantDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/$id',
  component: TenantDetailPage,
})

const webrootDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/$id/webroots/$webrootId',
  component: WebrootDetailPage,
})

const fqdnDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/$id/fqdns/$fqdnId',
  component: FQDNDetailPage,
})

const emailAccountDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/$id/email-accounts/$accountId',
  component: EmailAccountDetailPage,
})

const databaseDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/$id/databases/$databaseId',
  component: DatabaseDetailPage,
})

const valkeyDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/$id/valkey/$instanceId',
  component: ValkeyDetailPage,
})

const s3BucketDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/$id/s3-buckets/$bucketId',
  component: S3BucketDetailPage,
})

const webrootsRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/webroots',
  component: WebrootsPage,
})

const databasesRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/databases',
  component: DatabasesPage,
})

const zonesRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/zones',
  component: ZonesPage,
})

const zoneDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/zones/$id',
  component: ZoneDetailPage,
})

const valkeyRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/valkey',
  component: ValkeyPage,
})

const s3BucketsRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/s3-buckets',
  component: S3BucketsPage,
})

const platformConfigRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/platform-config',
  component: PlatformConfigPage,
})

const apiKeysRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/api-keys',
  component: APIKeysPage,
})

const auditLogRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/audit-log',
  component: AuditLogPage,
})

const routeTree = rootRoute.addChildren([
  loginRoute,
  authLayout.addChildren([
    dashboardRoute,
    brandsRoute,
    brandDetailRoute,
    regionsRoute,
    clustersRoute,
    tenantsRoute,
    createTenantRoute,
    tenantDetailRoute,
    webrootDetailRoute,
    fqdnDetailRoute,
    emailAccountDetailRoute,
    databaseDetailRoute,
    valkeyDetailRoute,
    s3BucketDetailRoute,
    webrootsRoute,
    databasesRoute,
    zonesRoute,
    zoneDetailRoute,
    valkeyRoute,
    s3BucketsRoute,
    platformConfigRoute,
    apiKeysRoute,
    auditLogRoute,
  ]),
])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
