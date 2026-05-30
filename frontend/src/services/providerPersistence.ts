export type ProviderSnapshot = {
  supportedModels?: Record<string, boolean>
  modelMapping?: Record<string, string>
}

export type SaveProvidersFn<T extends ProviderSnapshot> = (kind: string, providers: T[]) => Promise<void>

export const cloneProviderSnapshot = <T extends ProviderSnapshot>(provider: T): T => ({
  ...provider,
  supportedModels: provider.supportedModels ? { ...provider.supportedModels } : provider.supportedModels,
  modelMapping: provider.modelMapping ? { ...provider.modelMapping } : provider.modelMapping,
})

export const serializeProviderSnapshots = <T extends ProviderSnapshot>(providers: T[]): T[] =>
  providers.map((provider) => cloneProviderSnapshot(provider))

export const saveProviderSnapshot = async <T extends ProviderSnapshot>(
  kind: string,
  providers: T[],
  saveProviders: SaveProvidersFn<T>,
) => {
  await saveProviders(kind, serializeProviderSnapshots(providers))
}

export const providerPersistenceErrorMessage = (error: unknown, fallback: string) => {
  if (error instanceof Error && error.message) return error.message
  return fallback
}
