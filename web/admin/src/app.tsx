import { useState, useEffect } from 'react'
import { Outlet } from '@tanstack/react-router'
import { Sidebar } from '@/components/layout/sidebar'
import { CommandPalette } from '@/components/command-palette'

export function AuthLayout() {
  const [cmdOpen, setCmdOpen] = useState(false)

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setCmdOpen(o => !o)
      }
    }
    document.addEventListener('keydown', down)
    return () => document.removeEventListener('keydown', down)
  }, [])

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar onSearchClick={() => setCmdOpen(true)} />
      <main className="flex-1 overflow-y-auto">
        <div className="container mx-auto p-6">
          <Outlet />
        </div>
      </main>
      <CommandPalette open={cmdOpen} onOpenChange={setCmdOpen} />
    </div>
  )
}
