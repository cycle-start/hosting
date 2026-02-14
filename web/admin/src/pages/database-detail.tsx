import { useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Pencil, Users, ExternalLink } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate } from '@/lib/utils'
import { rules, validateField } from '@/lib/validation'
import {
  useDatabase, useDatabaseUsers,
  useCreateDatabaseUser, useUpdateDatabaseUser, useDeleteDatabaseUser,
  useCreateLoginSession,
} from '@/lib/hooks'
import type { DatabaseUser } from '@/lib/types'

const allPrivileges = ['SELECT', 'INSERT', 'UPDATE', 'DELETE', 'CREATE', 'DROP', 'ALTER', 'INDEX', 'REFERENCES', 'ALL']

export function DatabaseDetailPage() {
  const { id: tenantId, databaseId } = useParams({ from: '/auth/tenants/$id/databases/$databaseId' as never })

  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<DatabaseUser | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DatabaseUser | null>(null)
  const [touched, setTouched] = useState<Record<string, boolean>>({})

  // Create form
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [privileges, setPrivileges] = useState<string[]>(['SELECT', 'INSERT', 'UPDATE', 'DELETE'])

  // Edit form
  const [editPassword, setEditPassword] = useState('')
  const [editPrivileges, setEditPrivileges] = useState<string[]>([])

  const { data: database, isLoading } = useDatabase(databaseId)
  const { data: usersData, isLoading: usersLoading } = useDatabaseUsers(databaseId)
  const createMut = useCreateDatabaseUser()
  const updateMut = useUpdateDatabaseUser()
  const deleteMut = useDeleteDatabaseUser()
  const createLoginSessionMut = useCreateLoginSession()

  const handleOpenDbAdmin = async () => {
    try {
      const session = await createLoginSessionMut.mutateAsync(tenantId)
      const baseDomain = window.location.hostname.split('.').slice(1).join('.')
      window.open(
        `http://dbadmin.${baseDomain}/oauth2/start?rd=/&login_hint=${session.session_id}`,
        '_blank'
      )
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to open DB Admin')
    }
  }

  if (isLoading || !database) {
    return <div className="space-y-6"><Skeleton className="h-10 w-64" /><Skeleton className="h-64 w-full" /></div>
  }

  const touch = (f: string) => setTouched(p => ({ ...p, [f]: true }))
  const usernameErr = touched['username'] ? validateField(username, [rules.required(), rules.slug()]) : null
  const passwordErr = touched['password'] ? validateField(password, [rules.required(), rules.minLength(8)]) : null

  const togglePrivilege = (priv: string, list: string[], setter: (v: string[]) => void) => {
    setter(list.includes(priv) ? list.filter(p => p !== priv) : [...list, priv])
  }

  const handleCreate = async () => {
    setTouched({ username: true, password: true })
    if (validateField(username, [rules.required(), rules.slug()]) || validateField(password, [rules.required(), rules.minLength(8)]) || privileges.length === 0) return
    try {
      await createMut.mutateAsync({ database_id: databaseId, username, password, privileges })
      toast.success('User created'); setCreateOpen(false); setUsername(''); setPassword(''); setPrivileges(['SELECT', 'INSERT', 'UPDATE', 'DELETE']); setTouched({})
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const openEdit = (user: DatabaseUser) => {
    setEditTarget(user)
    setEditPassword('')
    setEditPrivileges([...user.privileges])
  }

  const handleUpdate = async () => {
    if (!editTarget) return
    const data: { id: string; password?: string; privileges?: string[] } = { id: editTarget.id }
    if (editPassword) {
      if (editPassword.length < 8) { toast.error('Password must be at least 8 characters'); return }
      data.password = editPassword
    }
    if (JSON.stringify(editPrivileges.sort()) !== JSON.stringify(editTarget.privileges.sort())) {
      if (editPrivileges.length === 0) { toast.error('Select at least one privilege'); return }
      data.privileges = editPrivileges
    }
    try {
      await updateMut.mutateAsync(data)
      toast.success('User updated'); setEditTarget(null)
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const columns: ColumnDef<DatabaseUser>[] = [
    { accessorKey: 'username', header: 'Username', cell: ({ row }) => <span className="font-medium">{row.original.username}</span> },
    {
      accessorKey: 'privileges', header: 'Privileges',
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1">
          {row.original.privileges.map(p => (
            <span key={p} className="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs">{p}</span>
          ))}
        </div>
      ),
    },
    { accessorKey: 'status', header: 'Status', cell: ({ row }) => <StatusBadge status={row.original.status} /> },
    {
      id: 'actions',
      cell: ({ row }) => (
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); openEdit(row.original) }}>
            <Pencil className="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteTarget(row.original) }}>
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        </div>
      ),
    },
  ]

  const PrivilegeCheckboxes = ({ selected, toggle }: { selected: string[]; toggle: (p: string) => void }) => (
    <div className="flex flex-wrap gap-2">
      {allPrivileges.map(p => (
        <label key={p} className="flex items-center gap-1.5 cursor-pointer">
          <input type="checkbox" checked={selected.includes(p)} onChange={() => toggle(p)} className="rounded border-input" />
          <span className="text-sm">{p}</span>
        </label>
      ))}
    </div>
  )

  return (
    <div className="space-y-6">
      <Breadcrumb segments={[
        { label: 'Tenants', href: '/tenants' },
        { label: tenantId, href: `/tenants/${tenantId}` },
        { label: 'Databases', href: `/tenants/${tenantId}`, hash: 'databases' },
        { label: database.name },
      ]} />

      <ResourceHeader
        title={database.name}
        subtitle={`Shard: ${database.shard_name || database.shard_id || '-'}`}
        status={database.status}
        actions={
          <Button variant="outline" size="sm" onClick={handleOpenDbAdmin}>
            <ExternalLink className="mr-2 h-4 w-4" />
            Open in DB Admin
          </Button>
        }
      />

      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>ID: <code>{database.id}</code></span>
        <CopyButton value={database.id} />
        <span className="ml-4">Created: {formatDate(database.created_at)}</span>
      </div>

      <div>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Database Users</h2>
          <Button size="sm" onClick={() => { setUsername(''); setPassword(''); setPrivileges(['SELECT', 'INSERT', 'UPDATE', 'DELETE']); setTouched({}); setCreateOpen(true) }}>
            <Plus className="mr-2 h-4 w-4" /> Add User
          </Button>
        </div>
        {!usersLoading && (usersData?.items?.length ?? 0) === 0 ? (
          <EmptyState icon={Users} title="No database users" description="Create a user to access this database." action={{ label: 'Add User', onClick: () => setCreateOpen(true) }} />
        ) : (
          <DataTable columns={columns} data={usersData?.items ?? []} loading={usersLoading} searchColumn="username" searchPlaceholder="Search users..." />
        )}
      </div>

      {/* Create User */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Database User</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Username</Label>
              <Input placeholder="app_user" value={username} onChange={(e) => setUsername(e.target.value)} onBlur={() => touch('username')} />
              {usernameErr && <p className="text-xs text-destructive">{usernameErr}</p>}
            </div>
            <div className="space-y-2">
              <Label>Password</Label>
              <Input type="password" placeholder="Min 8 characters" value={password} onChange={(e) => setPassword(e.target.value)} onBlur={() => touch('password')} />
              {passwordErr && <p className="text-xs text-destructive">{passwordErr}</p>}
            </div>
            <div className="space-y-2">
              <Label>Privileges</Label>
              <PrivilegeCheckboxes selected={privileges} toggle={(p) => togglePrivilege(p, privileges, setPrivileges)} />
              {privileges.length === 0 && <p className="text-xs text-destructive">Select at least one privilege</p>}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={createMut.isPending}>
              {createMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit User */}
      <Dialog open={!!editTarget} onOpenChange={(o) => !o && setEditTarget(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>Edit User: {editTarget?.username}</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>New Password (leave empty to keep current)</Label>
              <Input type="password" placeholder="Min 8 characters" value={editPassword} onChange={(e) => setEditPassword(e.target.value)} />
              {editPassword && editPassword.length < 8 && <p className="text-xs text-destructive">Must be at least 8 characters</p>}
            </div>
            <div className="space-y-2">
              <Label>Privileges</Label>
              <PrivilegeCheckboxes selected={editPrivileges} toggle={(p) => togglePrivilege(p, editPrivileges, setEditPrivileges)} />
              {editPrivileges.length === 0 && <p className="text-xs text-destructive">Select at least one privilege</p>}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditTarget(null)}>Cancel</Button>
            <Button onClick={handleUpdate} disabled={updateMut.isPending}>
              {updateMut.isPending ? 'Saving...' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete User */}
      <ConfirmDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)} title="Delete Database User"
        description={`Delete user "${deleteTarget?.username}"? They will lose all database access.`}
        confirmLabel="Delete" variant="destructive" loading={deleteMut.isPending}
        onConfirm={async () => { try { await deleteMut.mutateAsync(deleteTarget!.id); toast.success('User deleted'); setDeleteTarget(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />
    </div>
  )
}
