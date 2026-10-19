export function Logo({ className }: { className?: string }) {
  return (
    <div className={`flex items-center gap-3 ${className ?? ""}`.trim()}>
      <svg width="40" height="40" viewBox="0 0 32 32" aria-hidden="true">
        <rect x="4" y="7" width="4.5" height="18" rx="1.2" fill="#c9a227" />
        <rect x="10.5" y="7" width="4.5" height="18" rx="1.2" fill="#ece7df" opacity=".85" />
        <rect x="17" y="7" width="4.5" height="18" rx="1.2" fill="#ece7df" opacity=".85" />
        <rect x="17" y="7" width="4.5" height="6" rx="1.2" fill="#c9a227" />
        <rect x="23.5" y="7" width="4.5" height="18" rx="1.2" fill="#ece7df" opacity=".35" />
      </svg>
      <span className="font-serif font-semibold text-2xl">Shelfmark</span>
    </div>
  );
}
