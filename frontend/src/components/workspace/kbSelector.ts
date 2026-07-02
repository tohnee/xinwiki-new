/**
 * kbSelector - pure helpers for the Workspace header KB selector.
 *
 * P2 fix: the header `.kb-selector` was a dead display element - it
 * showed the active KB name with a chevron icon that implied a
 * dropdown, but clicking did nothing. These helpers power the new
 * dropdown: which items to render, how to label them, and which one
 * is the active row.
 */
import type { KbListItem } from './kbStore'

/**
 * Items to render in the KB dropdown. We always show at least the
 * "管理知识库" entry so the user can escape to the KB list page even
 * when they have zero KBs.
 */
export interface KbDropdownItem {
  id: string
  name: string
  isActive: boolean
}

export const buildDropdownItems = (
  kbList: readonly KbListItem[] | null | undefined,
  activeKbId: string | undefined | null,
): KbDropdownItem[] => {
  if (!Array.isArray(kbList) || kbList.length === 0) return []
  return kbList.map(kb => ({
    id: kb.id,
    name: kb.name,
    isActive: kb.id === activeKbId,
  }))
}

/**
 * The header chevron should rotate when the dropdown is open. This
 * keeps the rotation class logic in one testable place.
 */
export const chevronClass = (open: boolean): string => (open ? 'chevron-icon open' : 'chevron-icon')

/**
 * Whether the selector chip should be clickable. It is interactive
 * only when there is at least one KB to switch to.
 */
export const selectorIsInteractive = (
  kbList: readonly KbListItem[] | null | undefined,
): boolean => Array.isArray(kbList) && kbList.length > 0

/**
 * The aria-label for the selector, used by screen readers. Reflects
 * whether the dropdown will open or close.
 */
export const selectorAriaLabel = (
  open: boolean,
  activeKbName: string | undefined | null,
): string => {
  const name = activeKbName || '选择知识库'
  return open ? `关闭知识库选择器，当前：${name}` : `打开知识库选择器，当前：${name}`
}
