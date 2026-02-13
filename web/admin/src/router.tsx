import { createRouter, createRootRoute, createRoute, redirect, Outlet } from '@tanstack/react-router'
import { api } from '@/lib/api'
import { AuthLayout } from './app'
import { LoginPage } from './pages/login'
import { DashboardPage } from './pages/dashboard'
import { RegionsPage } from './pages/regions'
import { ClustersPage } from './pages/clusters'
import { TenantsPage } from './pages/tenants'
import { TenantDetailPage } from './pages/tenant-detail'
import { DatabasesPage } from './pages/databases'
import { ZonesPage } from './pages/zones'
import { ZoneDetailPage } from './pages/zone-detail'
import { ValkeyPage } from './pages/valkey'
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

const tenantDetailRoute = createRoute({
  getParentRoute: () => authLayout,
  path: '/tenants/$id',
  component: TenantDetailPage,
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
    regionsRoute,
    clustersRoute,
    tenantsRoute,
    tenantDetailRoute,
    databasesRoute,
    zonesRoute,
    zoneDetailRoute,
    valkeyRoute,
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
