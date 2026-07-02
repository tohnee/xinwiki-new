import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildInitialProgressSteps,
  advanceProgressSteps,
  completeAllSteps,
  hasThinkingContent,
  GENERATION_PROGRESS_STEPS,
} from './thinkingSteps.ts'

test('buildInitialProgressSteps returns 4 steps with step 0 running and the rest pending', () => {
  const steps = buildInitialProgressSteps()
  assert.equal(steps.length, 4)
  assert.equal(steps[0].status, 'running')
  assert.equal(steps[1].status, 'pending')
  assert.equal(steps[2].status, 'pending')
  assert.equal(steps[3].status, 'pending')
})

test('buildInitialProgressSteps assigns each step a unique id from the spec', () => {
  const steps = buildInitialProgressSteps()
  const ids = steps.map(s => s.id)
  assert.deepEqual(ids, GENERATION_PROGRESS_STEPS.map(s => s.id))
})

test('advanceProgressSteps marks the first N as completed and the next as running', () => {
  const initial = buildInitialProgressSteps()
  const advanced = advanceProgressSteps(initial, 2)
  assert.equal(advanced[0].status, 'completed')
  assert.equal(advanced[1].status, 'completed')
  assert.equal(advanced[2].status, 'running')
  assert.equal(advanced[3].status, 'pending')
})

test('advanceProgressSteps marks all steps completed when N equals length', () => {
  const initial = buildInitialProgressSteps()
  const advanced = advanceProgressSteps(initial, 4)
  // when completedCount === length, every step satisfies i < completedCount,
  // so no step is left "running" - they are all "completed".
  assert.equal(advanced.every(s => s.status === 'completed'), true)
  assert.equal(advanced[3].status, 'completed')
})

test('advanceProgressSteps treats negative completedCount as 0', () => {
  const initial = buildInitialProgressSteps()
  const advanced = advanceProgressSteps(initial, -1)
  assert.equal(advanced[0].status, 'running')
  assert.equal(advanced[1].status, 'pending')
})

test('completeAllSteps marks every step as completed', () => {
  const initial = buildInitialProgressSteps()
  const completed = completeAllSteps(initial)
  assert.equal(completed.every(s => s.status === 'completed'), true)
})

test('hasThinkingContent is false for null / undefined / empty array', () => {
  assert.equal(hasThinkingContent(null), false)
  assert.equal(hasThinkingContent(undefined), false)
  assert.equal(hasThinkingContent([]), false)
})

test('hasThinkingContent is true when there is at least one step', () => {
  const steps = buildInitialProgressSteps()
  assert.equal(hasThinkingContent(steps), true)
})
