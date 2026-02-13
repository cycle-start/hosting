import { ReactNode } from 'react'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

interface ArraySectionProps<T> {
  title: string
  items: T[]
  onChange: (items: T[]) => void
  defaultItem: () => T
  renderItem: (item: T, index: number, onChange: (item: T) => void) => ReactNode
  addLabel?: string
}

export function ArraySection<T>({ title, items, onChange, defaultItem, renderItem, addLabel }: ArraySectionProps<T>) {
  const addItem = () => onChange([...items, defaultItem()])
  const removeItem = (index: number) => onChange(items.filter((_, i) => i !== index))
  const updateItem = (index: number, item: T) => {
    const updated = [...items]
    updated[index] = item
    onChange(updated)
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">{title} {items.length > 0 && <span className="text-muted-foreground">({items.length})</span>}</h3>
        <Button type="button" variant="outline" size="sm" onClick={addItem}>
          <Plus className="mr-1 h-3 w-3" /> {addLabel ?? `Add ${title.replace(/s$/, '')}`}
        </Button>
      </div>
      {items.map((item, i) => (
        <Card key={i}>
          <CardContent className="pt-4">
            <div className="flex gap-2">
              <div className="flex-1 space-y-3">
                {renderItem(item, i, (updated) => updateItem(i, updated))}
              </div>
              <Button type="button" variant="ghost" size="icon" className="shrink-0 mt-0" onClick={() => removeItem(i)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
