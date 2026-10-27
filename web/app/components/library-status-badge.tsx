import { Badge } from "./badge";

// LibraryStatusBadge explains why a library has no categories, from its
// categorization status. Statuses with nothing to explain render "—".
export function LibraryStatusBadge({ status }: { status: string }) {
  switch (status) {
    case "needs_manual":
      return <Badge tone="neutral">Needs review</Badge>;
    case "failed":
      return <Badge tone="destructive">Failed</Badge>;
    case "pending":
    case "processing":
      return <Badge tone="neutral">Processing</Badge>;
    default:
      return <>—</>;
  }
}
