import type { ReactNode } from "react";

/** Общая обёртка экранов входа и регистрации: знак, заголовок, подзаголовок. */
export function AuthCard({
  title,
  subtitle,
  children,
  footer,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
  footer: ReactNode;
}) {
  return (
    <div className="flex w-full max-w-sm flex-col gap-6">
      <div className="flex flex-col items-center gap-3 text-center">
        <Logo />
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold tracking-tight text-balance">{title}</h1>
          <p className="text-sm text-fg-muted text-balance">{subtitle}</p>
        </div>
      </div>

      {children}

      <div className="text-center text-sm text-fg-muted">{footer}</div>
    </div>
  );
}

function Logo() {
  return (
    <div className="grid size-9 place-items-center rounded-lg bg-accent-solid text-accent-fg">
      <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden>
        <path
          d="M4 4v5.5a5 5 0 0 0 10 0V4"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
        />
      </svg>
    </div>
  );
}
