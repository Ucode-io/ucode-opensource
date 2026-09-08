/**
 * Знак ucode: «U»-скоба и точка сверху справа. Общий с favicon
 * (`public/favicon.svg` — та же геометрия, но с зашитыми hex вместо
 * токенов: файл открывается вне каскада приложения, переменных там нет).
 *
 * `haloColor` — фон, на который кладут знак: он отделяет точку от
 * ножки. Дефолт `--color-bg` подходит для страницы и сайдбара; экран
 * входа рисует его на серой плашке и передаёт `--color-surface-active`.
 */
export function BrandMark({
  size = 40,
  haloColor = "var(--color-bg)",
}: {
  size?: number;
  haloColor?: string;
}) {
  return (
    <svg width={size} height={size} viewBox="0 0 100 100" fill="none" aria-hidden>
      <defs>
        <linearGradient id="brand-mark-u" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--color-brand-gradient-from)" />
          <stop offset="100%" stopColor="var(--color-brand-gradient-to)" />
        </linearGradient>
      </defs>
      <path
        d="M28 18v42a22 22 0 0 0 44 0V36"
        stroke="url(#brand-mark-u)"
        strokeWidth="24"
        strokeLinecap="round"
      />
      {/* Точка на той же вертикали, что и правая ножка (cx = 72, как
          в path выше) — так ширина знака держится построением. */}
      <circle cx="72" cy="20" r="17" fill={haloColor} />
      <circle cx="72" cy="20" r="12" fill="var(--color-accent-solid)" />
    </svg>
  );
}
