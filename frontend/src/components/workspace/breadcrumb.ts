/**
 * Breadcrumb computation for the three-column Workspace.
 *
 * Extracted from views/workspace/Workspace.vue so the logic can be
 * unit-tested without mounting Vue. The component imports
 * `buildBreadcrumb()` and passes the result down to WorkspaceContent.
 */

export type BreadcrumbMode = 'chat' | 'page'

export interface BreadcrumbInput {
  mode: BreadcrumbMode
  /** Present when viewing a wiki page. */
  pageTitle?: string
  /** Optional KB name to make the first segment specific. */
  kbName?: string
}

/**
 * Build the breadcrumb trail for the workspace content header.
 *
 * - In `page` mode with a title, the trail is `[<kb>, <pageTitle>]`.
 * - Otherwise (chat mode, or page mode without a title), the trail
 *   falls back to `['XinWiki', '智能问答']` so the header never
 *   renders an empty breadcrumb.
 *
 * Previously the component computed this but never propagated it to
 * WorkspaceContent - the panel always received `[]` (P1 bug).
 */
export const buildBreadcrumb = (input: BreadcrumbInput): string[] => {
  const { mode, pageTitle, kbName } = input
  if (mode === 'page' && pageTitle) {
    return [kbName || '知识库', pageTitle]
  }
  return ['XinWiki', '智能问答']
}
