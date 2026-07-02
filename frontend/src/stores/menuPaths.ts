/**
 * Menu item definitions and filter helpers.
 *
 * Extracted from stores/menu.ts so the path list and the top-menu
 * filter can be unit-tested without booting Pinia / i18n / auth.
 *
 * The store re-exports `buildMenuItems()` to construct its reactive
 * `menuArr`, keeping the data shape in one place. Any new top-level
 * navigation entry MUST be added here and covered by menu.test.ts.
 */

export interface MenuItem {
  title: string
  titleKey?: string
  icon: string
  path: string
  childrenPath?: string
  children?: Array<Record<string, any>>
}

/**
 * Canonical list of top-level navigation entries shown in the sidebar.
 * Order matters - it is the visual order in the UI.
 */
export const MENU_PATHS = [
  'creatChat',
  'knowledge-bases',
  'workspace',
  'agents',
  'integrations',
  'organizations',
  'settings',
  'logout',
] as const

export type MenuPath = (typeof MENU_PATHS)[number]

export const buildMenuItems = (): MenuItem[] => [
  {
    title: '',
    titleKey: 'menu.newChat',
    icon: 'prefixIcon',
    path: 'creatChat',
    childrenPath: 'chat',
    children: [],
  },
  { title: '', titleKey: 'menu.knowledgeBase', icon: 'zhishiku', path: 'knowledge-bases' },
  { title: '', titleKey: 'menu.workspace', icon: 'workspace', path: 'workspace' },
  { title: '', titleKey: 'menu.agents', icon: 'agent', path: 'agents' },
  { title: '', titleKey: 'menu.integrations', icon: 'integration', path: 'integrations' },
  { title: '', titleKey: 'menu.organizations', icon: 'organization', path: 'organizations' },
  { title: '', titleKey: 'menu.settings', icon: 'setting', path: 'settings' },
  { title: '', titleKey: 'menu.logout', icon: 'logout', path: 'logout' },
]

/**
 * Top-menu whitelist: which entries show in the upper section of the
 * sidebar vs. the bottom section (settings/logout).
 *
 * `workspace` MUST be in this list - without it the NotebookLM-style
 * three-column workspace is unreachable from the navigation (P0 bug).
 */
export const TOP_MENU_PATHS: ReadonlySet<string> = new Set([
  'creatChat',
  'knowledge-bases',
  'workspace',
  'agents',
  'integrations',
  'organizations',
])

export const isTopMenuItem = (path: string): boolean => TOP_MENU_PATHS.has(path)

/**
 * Lite edition hides these paths (no org management, no logout).
 */
export const LITE_HIDDEN_PATHS: ReadonlySet<string> = new Set([
  'logout',
  'organizations',
])
