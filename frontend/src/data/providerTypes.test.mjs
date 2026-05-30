import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { Buffer } from 'node:buffer'
import ts from 'typescript'

const loadProviderTypes = async () => {
  const source = await readFile(new URL('./providerTypes.ts', import.meta.url), 'utf8')
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  const encoded = Buffer.from(outputText).toString('base64')
  return import(`data:text/javascript;base64,${encoded}`)
}

const loadFallbackIcons = async () => {
  const source = await readFile(new URL('../icons/fallbackLobeIcons.ts', import.meta.url), 'utf8')
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  const encoded = Buffer.from(outputText).toString('base64')
  return import(`data:text/javascript;base64,${encoded}`)
}

const loadLocale = async (locale) => {
  const source = await readFile(new URL(`../locales/${locale}.json`, import.meta.url), 'utf8')
  return JSON.parse(source)
}

test('provider type menu exposes only custom and DeepSeek', async () => {
  const { defaultProviderType, providerTypeOptions } = await loadProviderTypes()
  assert.deepEqual(
    providerTypeOptions.map((option) => option.value),
    ['custom', 'deepseek'],
  )
  assert.equal(providerTypeOptions[0].labelKey, 'components.main.form.providerTypes.custom')
  assert.equal(providerTypeOptions[0].descriptionKey, 'components.main.form.providerTypeDescriptions.custom')
  assert.equal(providerTypeOptions[1].labelKey, 'components.main.form.providerTypes.deepseek')
  assert.equal(providerTypeOptions[1].descriptionKey, 'components.main.form.providerTypeDescriptions.deepseek')
  assert.equal(defaultProviderType, 'custom')
})

test('provider type normalization maps legacy icon values to custom', async () => {
  const { normalizeProviderType } = await loadProviderTypes()
  assert.equal(normalizeProviderType('DeepSeek'), 'deepseek')
  assert.equal(normalizeProviderType(' custom '), 'custom')
  assert.equal(normalizeProviderType('aicoding'), 'custom')
  assert.equal(normalizeProviderType(''), 'custom')
})

test('provider protocol copy explains forwarding and conversion behavior', async () => {
  const zh = await loadLocale('zh')
  const en = await loadLocale('en')

  assert.equal(zh.components.main.form.labels.providerType, '接口兼容模式')
  assert.equal(zh.components.main.form.providerTypes.custom, '原样转发')
  assert.match(zh.components.main.form.hints.providerType, /协议转换/)
  assert.match(zh.components.main.form.providerTypeDescriptions.deepseek, /Codex/)

  assert.equal(en.components.main.form.labels.providerType, 'API compatibility mode')
  assert.equal(en.components.main.form.providerTypes.custom, 'Pass through')
  assert.match(en.components.main.form.hints.providerType, /protocol conversion/)
  assert.match(en.components.main.form.providerTypeDescriptions.deepseek, /Codex/)
})

test('pass-through provider type has a default icon', async () => {
  const { default: fallbackIcons } = await loadFallbackIcons()

  assert.match(fallbackIcons.custom, /<svg/)
  assert.match(fallbackIcons.custom, /currentColor/)
})
