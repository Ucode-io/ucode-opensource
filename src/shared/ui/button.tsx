import type { ComponentProps } from "react";

/**
 * Цвета и размеры — только токенами из app/styles.css.
 *
 * primary заливается не самим брендом, а тёмной ступенью его шкалы:
 * белый текст на #45aeff даёт 2.40:1 при нужных 4.5:1, на #0075cf — 4.72:1.
 * Бренд при этом остаётся брендом — в фокусе, активном состоянии и акцентах.
 */
const variants = {
  primary: "bg-accent-solid text-accent-fg hover:bg-accent-solid-hover",
  secondary: "bg-surface text-fg border border-border-strong hover:bg-surface-hover",
  ghost: "bg-transparent text-fg hover:bg-surface-hover",
  danger: "bg-transparent text-danger hover:bg-danger-subtle",
} as const;

const sizes = {
  sm: "h-7 px-2 text-xs gap-1",
  md: "h-(--spacing-control) px-3 text-sm gap-1.5",
} as const;

export function Button({
  variant = "primary",
  size = "md",
  className = "",
  ...props
}: ComponentProps<"button"> & {
  variant?: keyof typeof variants;
  size?: keyof typeof sizes;
}) {
  return (
    <button
      {...props}
      className={`inline-flex items-center justify-center rounded-md font-medium transition-colors disabled:pointer-events-none disabled:opacity-50 ${sizes[size]} ${variants[variant]} ${className}`}
    />
  );
}
