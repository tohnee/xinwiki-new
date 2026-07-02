import assert from 'node:assert/strict'
import test from 'node:test'

import { buildBreadcrumb } from './breadcrumb.ts'

test('buildBreadcrumb returns [kb, pageTitle] in page mode with a title', () => {
  assert.deepEqual(
    buildBreadcrumb({ mode: 'page', pageTitle: '架构设计', kbName: '产品文档' }),
    ['产品文档', '架构设计'],
  )
})

test('buildBreadcrumb falls back to default kb label when kbName omitted', () => {
  assert.deepEqual(
    buildBreadcrumb({ mode: 'page', pageTitle: '架构设计' }),
    ['知识库', '架构设计'],
  )
})

test('buildBreadcrumb returns XinWiki trail in chat mode (P1: no more empty breadcrumb)', () => {
  assert.deepEqual(
    buildBreadcrumb({ mode: 'chat' }),
    ['XinWiki', '智能问答'],
  )
})

test('buildBreadcrumb falls back to chat trail when page mode has no title', () => {
  assert.deepEqual(
    buildBreadcrumb({ mode: 'page' }),
    ['XinWiki', '智能问答'],
  )
})
