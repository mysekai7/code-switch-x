export const defaultProviderType = 'custom'

export const providerTypeOptions = [
  {
    value: 'custom',
    labelKey: 'components.main.form.providerTypes.custom',
    descriptionKey: 'components.main.form.providerTypeDescriptions.custom',
  },
  {
    value: 'deepseek',
    labelKey: 'components.main.form.providerTypes.deepseek',
    descriptionKey: 'components.main.form.providerTypeDescriptions.deepseek',
  },
] as const

export const normalizeProviderType = (value: string) => {
  const normalized = value?.toString().trim().toLowerCase() ?? ''
  if (normalized === 'deepseek') {
    return 'deepseek'
  }
  return defaultProviderType
}
