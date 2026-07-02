/**
 * mobilePanel - pure helpers for the Workspace right-panel mobile state.
 *
 * P2 fix: WorkspaceRightPanel used `v-if="visible && !isMobile"` and
 * `v-if="!visible && !isMobile"`, which both excluded mobile entirely.
 * The right panel was therefore unreachable on phones. These helpers
 * decide which surface to render (desktop panel / desktop handle /
 * mobile drawer / mobile FAB) so the logic is unit-testable.
 */
export interface PanelSurfaceState {
  /** Desktop: the always-visible side panel. */
  showDesktopPanel: boolean
  /** Desktop: the thin "open" handle shown when the panel is collapsed. */
  showDesktopHandle: boolean
  /** Mobile: the slide-in drawer overlay. */
  showMobileDrawer: boolean
  /** Mobile: the floating action button that opens the drawer. */
  showMobileFab: boolean
}

/**
 * Resolve which panel surfaces should render for the current viewport
 * + visibility state.
 *
 * - Desktop, visible  → show the panel, hide the handle, hide mobile.
 * - Desktop, hidden   → hide the panel, show the handle, hide mobile.
 * - Mobile,  visible  → show the drawer, hide the FAB, hide desktop.
 * - Mobile,  hidden   → hide the drawer, show the FAB, hide desktop.
 */
export const resolvePanelSurfaceState = (
  isMobile: boolean,
  visible: boolean,
): PanelSurfaceState => {
  if (isMobile) {
    return {
      showDesktopPanel: false,
      showDesktopHandle: false,
      showMobileDrawer: visible,
      showMobileFab: !visible,
    }
  }
  return {
    showDesktopPanel: visible,
    showDesktopHandle: !visible,
    showMobileDrawer: false,
    showMobileFab: false,
  }
}

/**
 * Accessibility label for the mobile FAB. Reflects what it will do
 * when tapped (open the panel).
 */
export const mobileFabAriaLabel = '打开生成面板'

/**
 * Accessibility label for the drawer close button.
 */
export const mobileDrawerCloseAriaLabel = '关闭生成面板'
