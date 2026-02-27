import type { ValidationError, StepID } from './types'

/**
 * Maps a validation error field path to the step it belongs to.
 */
export function fieldToStep(field: string): StepID {
  if (field === 'deploy_mode') return 'deploy_mode'
  if (field === 'region_name' || field === 'cluster_name') return 'region'
  if (field.startsWith('brand.')) return 'brand'
  if (field.startsWith('control_plane.')) return 'control_plane'
  if (field.startsWith('nodes')) return 'nodes'
  if (field.startsWith('tls.') || field === 'tls') return 'tls'
  return 'review'
}

/**
 * Get errors relevant to a specific step.
 */
export function errorsForStep(errors: ValidationError[], step: StepID): ValidationError[] {
  return errors.filter((e) => fieldToStep(e.field) === step)
}

/**
 * Get the error message for a specific field, or undefined if none.
 */
export function fieldError(errors: ValidationError[], field: string): string | undefined {
  return errors.find((e) => e.field === field)?.message
}
