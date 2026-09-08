import type { ReactNode } from "react";

/** Горизонтальная линия с подписью посередине. */
export function Divider({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <span className="h-px flex-1 bg-border" />
      <span className="text-xs text-fg-subtle">{children}</span>
      <span className="h-px flex-1 bg-border" />
    </div>
  );
}
