export type TreeNode = { kind: 'category' | 'document'; path: string; title: string; collapsed?: boolean; children?: TreeNode[] };
export type Heading = { id: string; text: string; level: number };
export type Doc = { path: string; title: string; description?: string; html: string; headings?: Heading[] };
export type SearchResult = { path: string; title: string; description?: string; snippet?: string };
export const documentURL = (path: string) => `/docs/${path.split('/').map(encodeURIComponent).join('/')}`;
export function documentNodes(nodes: TreeNode[]): TreeNode[] { return nodes.flatMap(node => node.kind === 'document' ? [node] : documentNodes(node.children ?? [])); }
