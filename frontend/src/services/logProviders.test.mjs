import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const readLogsService = () => readFile(new URL('./logs.ts', import.meta.url), 'utf8')

test('log provider filter options come from configured providers instead of historical logs', async () => {
  const source = await readLogsService()

  assert.match(source, /import \{ LoadProviders \} from '..\/..\/bindings\/codeswitch\/services\/providerservice'/)
  assert.match(source, /type ProviderKind = 'claude' \| 'codex'/)
  assert.match(source, /const allProviderKinds: ProviderKind\[\] = \['claude', 'codex'\]/)
  assert.match(source, /const providerKinds = isProviderKind\(platform\) \? \[platform\] : allProviderKinds/)
  assert.match(source, /Promise\.all\(providerKinds\.map\(\(kind\) => LoadProviders\(kind\)\)\)/)
  assert.match(source, /new Set<string>\(\)/)
  assert.doesNotMatch(source, /LogService\.ListProviders/)
})
