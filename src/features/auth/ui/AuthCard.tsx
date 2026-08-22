import type { ReactNode } from "react";
import { BrandMark } from "@/shared/ui/brand-mark";

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

/*
 * Плашка вокруг знака — тот же серый токен и тот же шум, что у панели
 * волн рядом (AuthLayout.Waves; id фильтра свой, иначе конфликт на
 * одном экране входа, где обе плашки видны разом). Шум рисуется под
 * знаком, а не над ним: знак поверх закрывает его собой и остаётся
 * чистым.
 */
function Logo() {
  return (
    <div className="relative grid h-14 w-16 place-items-center overflow-hidden rounded-2xl bg-surface-active">
      <svg className="pointer-events-none absolute inset-0 h-full w-full" aria-hidden>
        <filter id="auth-logo-noise" x="-20%" y="-20%" width="140%" height="140%">
          <feTurbulence
            type="fractalNoise"
            baseFrequency="0.65"
            numOctaves="2"
            seed="7"
            stitchTiles="stitch"
          />
          <feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0 0.6 0" />
        </filter>
        <rect width="100%" height="100%" filter="url(#auth-logo-noise)" style={{ mixBlendMode: "overlay" }} />
      </svg>

      <div className="relative">
        <BrandMark haloColor="var(--color-surface-active)" />
      </div>
    </div>
  );
}
