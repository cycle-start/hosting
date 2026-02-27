interface Props {
  error?: string
}

export function FieldError({ error }: Props) {
  if (!error) return null
  return <p className="text-xs text-destructive mt-1">{error}</p>
}
