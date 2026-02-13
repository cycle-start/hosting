type Rule = (value: string) => string | null

export const rules = {
  required: (): Rule => (v) => v.trim() ? null : 'Required',
  slug: (): Rule => (v) => /^[a-z][a-z0-9_-]{0,62}$/.test(v) ? null : 'Must start with a-z, only lowercase letters, digits, hyphens, underscores (max 63)',
  email: (): Rule => (v) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) ? null : 'Invalid email address',
  fqdn: (): Rule => (v) => /^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/.test(v) ? null : 'Invalid domain name',
  minLength: (n: number): Rule => (v) => v.length >= n ? null : `Must be at least ${n} characters`,
  maxLength: (n: number): Rule => (v) => v.length <= n ? null : `Must be at most ${n} characters`,
  min: (n: number): Rule => (v) => Number(v) >= n ? null : `Must be at least ${n}`,
  oneOf: (values: string[]): Rule => (v) => values.includes(v) ? null : `Must be one of: ${values.join(', ')}`,
  minItems: (n: number): Rule => (v) => v.split(',').filter(Boolean).length >= n ? null : `Select at least ${n}`,
}

export function validateField(value: string, fieldRules: Rule[]): string | null {
  for (const rule of fieldRules) {
    const err = rule(value)
    if (err) return err
  }
  return null
}

export function useFormValidation<T extends Record<string, string>>(
  values: T,
  schema: Partial<Record<keyof T, Rule[]>>
): { errors: Partial<Record<keyof T, string | null>>; isValid: boolean } {
  const errors: Partial<Record<keyof T, string | null>> = {}
  let isValid = true
  for (const [key, fieldRules] of Object.entries(schema)) {
    if (!fieldRules) continue
    const err = validateField(values[key] ?? '', fieldRules as Rule[])
    errors[key as keyof T] = err
    if (err) isValid = false
  }
  return { errors, isValid }
}
