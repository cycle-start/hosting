import { Outlet } from '@tanstack/react-router'
import { Sidebar } from '@/components/layout/sidebar'

export function AuthLayout() {
  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
        <div className="container mx-auto p-6">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
