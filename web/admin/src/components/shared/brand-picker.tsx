import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { useBrands } from '@/lib/hooks'

interface BrandPickerProps {
  value: string[]
  onChange: (brands: string[]) => void
}

export function BrandPicker({ value, onChange }: BrandPickerProps) {
  const { data, isLoading } = useBrands({ limit: 100 })
  const brands = data?.items ?? []
  const isAll = value.includes('*')

  const toggleAll = () => {
    if (isAll) {
      onChange([])
    } else {
      onChange(['*'])
    }
  }

  const toggleBrand = (brandId: string) => {
    if (isAll) {
      // Switching from wildcard: select all brands except this one
      onChange(brands.filter(b => b.id !== brandId).map(b => b.id))
    } else if (value.includes(brandId)) {
      onChange(value.filter(b => b !== brandId))
    } else {
      onChange([...value, brandId])
    }
  }

  if (isLoading) {
    return <Skeleton className="h-20 w-full" />
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center space-x-2">
        <Checkbox
          id="brand-all"
          checked={isAll}
          onCheckedChange={toggleAll}
        />
        <Label htmlFor="brand-all" className="font-medium">
          All brands (platform admin)
        </Label>
      </div>

      {!isAll && brands.length > 0 && (
        <div className="space-y-1.5 pl-2">
          {brands.map(brand => (
            <div key={brand.id} className="flex items-center space-x-2">
              <Checkbox
                id={`brand-${brand.id}`}
                checked={value.includes(brand.id)}
                onCheckedChange={() => toggleBrand(brand.id)}
              />
              <Label htmlFor={`brand-${brand.id}`} className="text-sm">
                {brand.name} <span className="text-muted-foreground">({brand.id})</span>
              </Label>
            </div>
          ))}
        </div>
      )}

      {!isAll && brands.length === 0 && (
        <p className="text-sm text-muted-foreground pl-2">No brands found. Create a brand first.</p>
      )}
    </div>
  )
}
