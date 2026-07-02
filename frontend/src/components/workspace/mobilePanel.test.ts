import assert from 'node:assert/strict'
import test from 'node:test'

import {
  resolvePanelSurfaceState,
  mobileFabAriaLabel,
  mobileDrawerCloseAriaLabel,
} from './mobilePanel.ts'

test('desktop + visible: only the desktop panel shows', () => {
  const s = resolvePanelSurfaceState(false, true)
  assert.equal(s.showDesktopPanel, true)
  assert.equal(s.showDesktopHandle, false)
  assert.equal(s.showMobileDrawer, false)
  assert.equal(s.showMobileFab, false)
})

test('desktop + hidden: only the desktop handle shows', () => {
  const s = resolvePanelSurfaceState(false, false)
  assert.equal(s.showDesktopPanel, false)
  assert.equal(s.showDesktopHandle, true)
  assert.equal(s.showMobileDrawer, false)
  assert.equal(s.showMobileFab, false)
})

test('mobile + visible: only the mobile drawer shows (P2 fix: panel was unreachable on mobile)', () => {
  const s = resolvePanelSurfaceState(true, true)
  assert.equal(s.showDesktopPanel, false)
  assert.equal(s.showDesktopHandle, false)
  assert.equal(s.showMobileDrawer, true)
  assert.equal(s.showMobileFab, false)
})

test('mobile + hidden: only the mobile FAB shows', () => {
  const s = resolvePanelSurfaceState(true, false)
  assert.equal(s.showDesktopPanel, false)
  assert.equal(s.showDesktopHandle, false)
  assert.equal(s.showMobileDrawer, false)
  assert.equal(s.showMobileFab, true)
})

test('mobileFabAriaLabel is a non-empty string', () => {
  assert.equal(typeof mobileFabAriaLabel, 'string')
  assert.ok(mobileFabAriaLabel.length > 0)
})

test('mobileDrawerCloseAriaLabel is a non-empty string', () => {
  assert.equal(typeof mobileDrawerCloseAriaLabel, 'string')
  assert.ok(mobileDrawerCloseAriaLabel.length > 0)
})
