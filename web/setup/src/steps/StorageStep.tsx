import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import type { Config } from '@/lib/types'
import { cn } from '@/lib/utils'
import { HardDrive, Cloud } from 'lucide-react'

interface Props {
  config: Config
  onChange: (config: Config) => void
}

export function StorageStep({ config, onChange }: Props) {
  const storage = config.storage

  const updateStorage = (updates: Partial<typeof storage>) => {
    onChange({ ...config, storage: { ...storage, ...updates } })
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Storage</h2>
        <p className="text-muted-foreground mt-1">
          Choose how object storage (S3) and file storage are provided.
        </p>
      </div>

      <div className="grid gap-4 max-w-lg">
        <div className="grid grid-cols-2 gap-3">
          {(['builtin', 'external'] as const).map((mode) => {
            const selected = storage.mode === mode
            const Icon = mode === 'builtin' ? HardDrive : Cloud
            return (
              <button
                key={mode}
                onClick={() => updateStorage({ mode })}
                className={cn(
                  'flex items-center gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent/50',
                  selected && 'border-primary bg-accent/50 ring-1 ring-primary'
                )}
              >
                <Icon className="h-5 w-5 shrink-0" />
                <div>
                  <div className="font-medium text-sm">
                    {mode === 'builtin' ? 'Built-in Ceph' : 'External S3'}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {mode === 'builtin'
                      ? 'S3 + CephFS on your nodes'
                      : 'MinIO, AWS S3, etc.'}
                  </div>
                </div>
              </button>
            )
          })}
        </div>

        {storage.mode === 'external' && (
          <div className="space-y-4 border-t pt-4">
            <div className="space-y-2">
              <Label htmlFor="s3_endpoint">S3 Endpoint</Label>
              <Input
                id="s3_endpoint"
                placeholder="https://s3.amazonaws.com"
                value={storage.s3_endpoint}
                onChange={(e) => updateStorage({ s3_endpoint: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label htmlFor="s3_access_key">Access Key</Label>
                <Input
                  id="s3_access_key"
                  value={storage.s3_access_key}
                  onChange={(e) =>
                    updateStorage({ s3_access_key: e.target.value })
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="s3_secret_key">Secret Key</Label>
                <Input
                  id="s3_secret_key"
                  type="password"
                  value={storage.s3_secret_key}
                  onChange={(e) =>
                    updateStorage({ s3_secret_key: e.target.value })
                  }
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="s3_bucket">Bucket Name</Label>
              <Input
                id="s3_bucket"
                placeholder="hosting-storage"
                value={storage.s3_bucket_name}
                onChange={(e) =>
                  updateStorage({ s3_bucket_name: e.target.value })
                }
              />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
