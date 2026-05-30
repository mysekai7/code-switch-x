import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { Buffer } from 'node:buffer'
import ts from 'typescript'

const loadModule = async () => {
  const source = await readFile(new URL('./providerSaveQueue.ts', import.meta.url), 'utf8')
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  })
  const encoded = Buffer.from(outputText).toString('base64')
  return import(`data:text/javascript;base64,${encoded}`)
}

test('provider save queue runs tasks in order', async () => {
  const { createSequentialTaskQueue } = await loadModule()
  const enqueue = createSequentialTaskQueue()
  const events = []
  let releaseFirst

  const first = enqueue(async () => {
    events.push('first:start')
    await new Promise((resolve) => {
      releaseFirst = resolve
    })
    events.push('first:end')
  })
  const second = enqueue(async () => {
    events.push('second:start')
    events.push('second:end')
  })
  const third = enqueue(async () => {
    events.push('third:start')
    events.push('third:end')
  })

  await Promise.resolve()
  assert.deepEqual(events, ['first:start'])

  releaseFirst()
  await Promise.all([first, second, third])

  assert.deepEqual(events, [
    'first:start',
    'first:end',
    'second:start',
    'second:end',
    'third:start',
    'third:end',
  ])
})

test('provider save queue continues after failure', async () => {
  const { createSequentialTaskQueue } = await loadModule()
  const enqueue = createSequentialTaskQueue()
  const events = []

  const first = enqueue(async () => {
    events.push('first:start')
    throw new Error('boom')
  })
  const second = enqueue(async () => {
    events.push('second:start')
  })

  await assert.rejects(first, /boom/)
  await second

  assert.deepEqual(events, ['first:start', 'second:start'])
})
