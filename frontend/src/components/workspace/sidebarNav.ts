/**
 * Pure helpers for the WorkspaceSidebar component.
 *
 * Extracted so the nav-item click routing, the "new page" action,
 * and the favorites/history placeholder behaviour can be unit-tested
 * without mounting Vue. The component imports these and feeds its
 * reactive state through them.
 */

export type SidebarNavId = 'chat' | 'knowledge' | 'favorites' | 'history'

export interface NavAction {
  /** New active nav id (which item should be highlighted). */
  activeNav: SidebarNavId
  /** If set, the sidebar should emit `select-page` with this value. */
  selectPage: 'clear' | null
  /** If true, the sidebar should emit `create-page` to spawn a new wiki page. */
  createPage: boolean
}

export const DEFAULT_NAV_ACTION: NavAction = {
  activeNav: 'chat',
  selectPage: null,
  createPage: false,
}

/**
 * Resolve a click on a sidebar nav item (问答 / 知识库 / 收藏夹 / 历史记录)
 * into the action the component should take.
 *
 * P1 bug: previously `favorites` and `history` only flipped `activeNav`
 * with no content behind them - the user clicked and got an empty panel.
 * We now mark them as `no-op` so the component can show an empty-state
 * placeholder instead of pretending there is content.
 *
 * P1 bug: the "新建" button at the top of the sidebar had no @click
 * handler at all - we now return `createPage: true` for the synthetic
 * `create` action so the parent can route to the page editor.
 */
export const resolveNavAction = (id: string): NavAction => {
  switch (id) {
    case 'chat':
      return { activeNav: 'chat', selectPage: 'clear', createPage: false }
    case 'knowledge':
      return { activeNav: 'knowledge', selectPage: null, createPage: false }
    case 'favorites':
    case 'history':
      // Empty-state nav: highlight the item, but emit nothing.
      // The component renders an empty-state placeholder for these.
      return { activeNav: id as SidebarNavId, selectPage: null, createPage: false }
    case 'create':
      return { activeNav: 'knowledge', selectPage: null, createPage: true }
    default:
      return DEFAULT_NAV_ACTION
  }
}

/**
 * Whether the given nav id has real content backing it (vs. an
 * empty-state placeholder). Used by the template to decide whether
 * to render the "暂未开放" empty card.
 */
export const navHasContent = (id: SidebarNavId): boolean => {
  return id === 'chat' || id === 'knowledge'
}
