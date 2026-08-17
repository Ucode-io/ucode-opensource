import type { ComponentProps, ReactNode } from "react";

const control =
  "h-(--spacing-input) w-full rounded-md border border-border-strong bg-surface px-2.5 text-sm text-fg outline-none transition-colors placeholder:text-fg-subtle hover:border-fg-subtle focus:border-accent disabled:opacity-50";

export function Input({ className = "", ...props }: ComponentProps<"input">) {
  return <input {...props} className={`${control} ${className}`} />;
}

export function Select({ className = "", ...props }: ComponentProps<"select">) {
  return <select {...props} className={`${control} ${className}`} />;
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium text-fg-muted">{label}</span>
      {children}
      {hint && <span className="text-xs text-fg-subtle">{hint}</span>}
    </label>
  );
}
