export type SequentialTaskQueue = <T>(task: () => Promise<T>) => Promise<T>

export const createSequentialTaskQueue = (): SequentialTaskQueue => {
  let tail: Promise<void> = Promise.resolve()

  return <T>(task: () => Promise<T>) => {
    const run = tail.then(task, task)
    tail = run.then(
      () => undefined,
      () => undefined,
    )
    return run
  }
}
