import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { Buffer } from 'node:buffer'
import ts from 'typescript'

const loadModule = async () => {
  const source = await readFile(new URL('./providerPersistence.ts', import.meta.url), 'utf8')
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  const encoded = Buffer.from(outputText).toString('base64')
  return import(`data:text/javascript;base64,${encoded}`)
}

test('saveProviderSnapshot propagates backend save failures', async () => {
  const { saveProviderSnapshot } = await loadModule()

  await assert.rejects(
    () => saveProviderSnapshot('claude', [], async () => {
      throw new Error('save exploded')
    }),
    /save exploded/,
  )
})

test('saveProviderSnapshot sends a detached provider snapshot', async () => {
  const { saveProviderSnapshot } = await loadModule()
  const provider = {
    id: 1,
    name: 'deepseek',
    supportedModels: { 'claude-*': true },
    modelMapping: { 'claude-*': 'deepseek-chat' },
  }
  let captured

  await saveProviderSnapshot('claude', [provider], async (_kind, providers) => {
    captured = providers
  })

  assert.notEqual(captured, [provider])
  assert.notEqual(captured[0], provider)
  assert.notEqual(captured[0].supportedModels, provider.supportedModels)
  assert.notEqual(captured[0].modelMapping, provider.modelMapping)
  assert.deepEqual(captured[0], provider)
})

test('providerPersistenceErrorMessage prefers backend error text', async () => {
  const { providerPersistenceErrorMessage } = await loadModule()

  assert.equal(
    providerPersistenceErrorMessage(new Error('配置验证失败'), '保存供应商失败'),
    '配置验证失败',
  )
  assert.equal(providerPersistenceErrorMessage('bad', '保存供应商失败'), '保存供应商失败')
})
